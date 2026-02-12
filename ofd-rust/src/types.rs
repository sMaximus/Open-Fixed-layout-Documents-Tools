//! OFD 文档类型定义

use serde::{Deserialize, Serialize};

/// OFD 文档根结构
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
#[serde(rename = "OFD")]
pub struct OFDDocument {
    #[serde(rename = "@Version", default)]
    pub version: String,
    #[serde(rename = "@DocType", default)]
    pub doc_type: String,
    #[serde(rename = "DocBody", default)]
    pub doc_body: Vec<DocBody>,
}

/// 文档体
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct DocBody {
    #[serde(rename = "DocInfo", default)]
    pub doc_info: DocInfo,
    #[serde(rename = "DocRoot", default)]
    pub doc_root: String,
}

/// 文档信息
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct DocInfo {
    #[serde(rename = "DocID", default)]
    pub doc_id: String,
    #[serde(rename = "Title", default)]
    pub title: String,
    #[serde(rename = "Author", default)]
    pub author: String,
    #[serde(rename = "CreationDate", default)]
    pub creation_date: String,
    #[serde(rename = "Creator", default)]
    pub creator: String,
}

/// Document.xml 结构
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
#[serde(rename = "Document")]
pub struct Document {
    #[serde(rename = "CommonData", default)]
    pub common_data: CommonData,
    #[serde(rename = "Pages", default)]
    pub pages: Pages,
}

/// 公共数据
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct CommonData {
    #[serde(rename = "MaxUnitID", default)]
    pub max_unit_id: i32,
    #[serde(rename = "PageArea", default)]
    pub page_area: PageArea,
    #[serde(rename = "PublicRes", default)]
    pub public_res: Vec<String>,
    #[serde(rename = "DocumentRes", default)]
    pub document_res: Vec<String>,
    #[serde(rename = "TemplatePage", default)]
    pub template_page: Vec<TemplatePageRef>,
}

/// 模板页引用（Document.xml 中的声明）
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct TemplatePageRef {
    #[serde(rename = "@ID", default)]
    pub id: String,
    #[serde(rename = "@BaseLoc", default)]
    pub base_loc: String,
}

/// 页面区域
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct PageArea {
    #[serde(rename = "PhysicalBox", default)]
    pub physical_box: String,
    #[serde(rename = "ApplicationBox", default)]
    pub application_box: String,
    #[serde(rename = "ContentBox", default)]
    pub content_box: String,
}

/// 页面列表
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct Pages {
    #[serde(rename = "Page", default)]
    pub page: Vec<PageRef>,
}

/// 页面引用
#[derive(Debug, Default, Clone, Deserialize, Serialize)]
pub struct PageRef {
    #[serde(rename = "@ID", default)]
    pub id: String,
    #[serde(rename = "@BaseLoc", default)]
    pub base_loc: String,
}
