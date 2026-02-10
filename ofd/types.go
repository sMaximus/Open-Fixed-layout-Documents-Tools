package ofd

import "encoding/xml"

// OFD 文档根结构
type OFDDocument struct {
	XMLName   xml.Name  `xml:"OFD"`
	Version   string    `xml:"Version,attr"`
	DocType   string    `xml:"DocType,attr"`
	DocBody   []DocBody `xml:"DocBody"`
}

// 文档体
type DocBody struct {
	DocInfo  DocInfo  `xml:"DocInfo"`
	DocRoot  string   `xml:"DocRoot"`
	Versions Versions `xml:"Versions"`
}

// 文档信息
type DocInfo struct {
	DocID        string   `xml:"DocID"`
	Title        string   `xml:"Title"`
	Author       string   `xml:"Author"`
	Subject      string   `xml:"Subject"`
	Abstract     string   `xml:"Abstract"`
	CreationDate string   `xml:"CreationDate"`
	ModDate      string   `xml:"ModDate"`
	Creator      string   `xml:"Creator"`
	CreatorVersion string `xml:"CreatorVersion"`
	Keywords     Keywords `xml:"Keywords"`
}

// 关键词
type Keywords struct {
	Keyword []string `xml:"Keyword"`
}

// 版本信息
type Versions struct {
	Version []Version `xml:"Version"`
}

type Version struct {
	ID      string `xml:"ID,attr"`
	Index   int    `xml:"Index,attr"`
	Current bool   `xml:"Current,attr"`
	BaseLoc string `xml:"BaseLoc"`
}

// Document.xml 结构
type Document struct {
	XMLName      xml.Name     `xml:"Document"`
	CommonData   CommonData   `xml:"CommonData"`
	Pages        Pages        `xml:"Pages"`
	Annotations  string       `xml:"Annotations"`
}

// 公共数据
type CommonData struct {
	MaxUnitID   int         `xml:"MaxUnitID"`
	PageArea    PageArea    `xml:"PageArea"`
	PublicRes   []string    `xml:"PublicRes"`
	DocumentRes []string    `xml:"DocumentRes"`
}

// 页面区域
type PageArea struct {
	PhysicalBox string `xml:"PhysicalBox"`
	ApplicationBox string `xml:"ApplicationBox"`
	ContentBox  string `xml:"ContentBox"`
}

// 页面列表
type Pages struct {
	Page []PageRef `xml:"Page"`
}

// 页面引用
type PageRef struct {
	ID      string `xml:"ID,attr"`
	BaseLoc string `xml:"BaseLoc"`
}
