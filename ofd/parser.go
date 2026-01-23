package ofd

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
)

// ParseResult OFD解析结果
type ParseResult struct {
	OFD       *OFDDocument
	Document  *Document
	Files     []string
	PageCount int
	Error     string
}

// Parser OFD解析器
type Parser struct {
	data      []byte
	reader    *zip.Reader
	ofd       *OFDDocument
	document  *Document
	files     []string
	fonts     map[string]Font
	fontFiles map[string][]byte
	images    map[string][]byte
}

// NewParser 创建解析器
func NewParser(data []byte) (*Parser, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	
	p := &Parser{
		data:   data,
		reader: reader,
		files:  make([]string, 0),
	}
	
	for _, f := range reader.File {
		p.files = append(p.files, f.Name)
	}
	
	return p, nil
}

// Parse 解析OFD文件
func (p *Parser) Parse() (*ParseResult, error) {
	result := &ParseResult{
		Files: p.files,
	}
	
	// 解析 OFD.xml
	ofdData, err := p.readFile("OFD.xml")
	if err != nil {
		result.Error = "无法读取OFD.xml: " + err.Error()
		return result, err
	}
	
	p.ofd = &OFDDocument{}
	if err := xml.Unmarshal(ofdData, p.ofd); err != nil {
		result.Error = "解析OFD.xml失败: " + err.Error()
		return result, err
	}
	result.OFD = p.ofd
	
	// 解析 Document.xml
	if len(p.ofd.DocBody) > 0 {
		docRoot := p.ofd.DocBody[0].DocRoot
		docPath := strings.TrimPrefix(docRoot, "/")
		
		docData, err := p.readFile(docPath)
		if err != nil {
			// 尝试其他可能的路径
			docData, err = p.readFile("Doc_0/Document.xml")
		}
		
		if err == nil {
			p.document = &Document{}
			if err := xml.Unmarshal(docData, p.document); err == nil {
				result.Document = p.document
				result.PageCount = len(p.document.Pages.Page)
			}
		}
	}
	
	return result, nil
}


// readFile 读取ZIP中的文件
func (p *Parser) readFile(name string) ([]byte, error) {
	name = strings.TrimPrefix(name, "/")
	nameLower := strings.ToLower(name)
	
	// 尝试精确匹配
	for _, f := range p.reader.File {
		if f.Name == name || strings.EqualFold(f.Name, name) {
			return p.readZipFile(f)
		}
	}
	
	// 尝试后缀匹配
	for _, f := range p.reader.File {
		fLower := strings.ToLower(f.Name)
		if strings.HasSuffix(fLower, nameLower) || strings.HasSuffix(fLower, "/"+nameLower) {
			return p.readZipFile(f)
		}
	}
	
	// 尝试基础文件名匹配
	baseName := path.Base(nameLower)
	for _, f := range p.reader.File {
		if strings.EqualFold(path.Base(f.Name), baseName) {
			return p.readZipFile(f)
		}
	}
	
	return nil, errors.New("文件不存在: " + name)
}

func (p *Parser) readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	
	return io.ReadAll(rc)
}

// GetFileContent 获取指定文件内容
func (p *Parser) GetFileContent(name string) ([]byte, error) {
	return p.readFile(name)
}

// GetFiles 获取所有文件列表
func (p *Parser) GetFiles() []string {
	return p.files
}

// GetDocInfo 获取文档信息
func (p *Parser) GetDocInfo() *DocInfo {
	if p.ofd != nil && len(p.ofd.DocBody) > 0 {
		return &p.ofd.DocBody[0].DocInfo
	}
	return nil
}

// GetPageCount 获取页数
func (p *Parser) GetPageCount() int {
	if p.document != nil {
		return len(p.document.Pages.Page)
	}
	return 0
}

// GetPagePath 获取页面文件路径
func (p *Parser) GetPagePath(index int) string {
	if p.document == nil || index >= len(p.document.Pages.Page) {
		return ""
	}
	
	pageRef := p.document.Pages.Page[index]
	pageLoc := strings.TrimPrefix(pageRef.BaseLoc, "/")
	
	// 获取文档根目录
	docBase := ""
	if p.ofd != nil && len(p.ofd.DocBody) > 0 {
		docRoot := p.ofd.DocBody[0].DocRoot
		docRoot = strings.TrimPrefix(docRoot, "/")
		docBase = path.Dir(docRoot)
	}
	
	// 尝试多种路径组合
	candidates := []string{
		path.Join(docBase, pageLoc),
		pageLoc,
		path.Join(docBase, "Pages", pageLoc),
	}
	
	// 查找实际存在的文件
	for _, candidate := range candidates {
		for _, f := range p.files {
			if strings.EqualFold(f, candidate) || strings.HasSuffix(f, "/"+path.Base(candidate)) {
				return f
			}
		}
	}
	
	// 按页面索引查找 Page_X/Content.xml 模式
	for _, f := range p.files {
		lower := strings.ToLower(f)
		pagePattern := fmt.Sprintf("page_%d/content.xml", index)
		if strings.Contains(lower, pagePattern) || strings.Contains(lower, fmt.Sprintf("page_%d.xml", index)) {
			return f
		}
	}
	
	return path.Join(docBase, pageLoc)
}

// GetPageSize 获取页面尺寸（不渲染内容）
func (p *Parser) GetPageSize(index int) (float64, float64) {
	if p.document == nil || index >= len(p.document.Pages.Page) {
		return 210, 297 // 默认 A4
	}

	// 获取页面路径
	pagePath := p.GetPagePath(index)
	if pagePath == "" {
		return 210, 297
	}

	// 读取页面XML
	pageData, err := p.readFile(pagePath)
	if err != nil {
		return 210, 297
	}

	// 移除命名空间前缀
	pageXML := removeNamespacePrefix(string(pageData))

	// 解析页面
	var page Page
	if err := xml.Unmarshal([]byte(pageXML), &page); err != nil {
		return 210, 297
	}

	// 获取页面尺寸
	if page.Area.PhysicalBox != "" {
		return parseBox(page.Area.PhysicalBox)
	}
	if p.document != nil && p.document.CommonData.PageArea.PhysicalBox != "" {
		return parseBox(p.document.CommonData.PageArea.PhysicalBox)
	}

	return 210, 297
}
