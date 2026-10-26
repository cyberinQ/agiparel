package extract

import (
 "archive/zip"
 "bytes"
 "context"
 "strings"
 "testing"
)

func office(parts map[string]string) []byte {
 var b bytes.Buffer; z:=zip.NewWriter(&b)
 for name,content:=range parts {w,e:=z.Create(name);if e!=nil{panic(e)};_,_=w.Write([]byte(content))};_ = z.Close();return b.Bytes()
}
func TestCSVExactRowsAndCoordinates(t *testing.T){
 raw:=[]byte("PO,Qty\nREP3,57600\nREP4,72000\n")
 r:=Process(context.Background(),"orders.csv",raw)
 if r.Status!="EXTRACTED"||len(r.Segments)!=3||!strings.Contains(string(r.Segments[2].Locator),`"row":3`) {t.Fatalf("%+v",r)}
}
func TestDOCXParagraphs(t *testing.T){
 raw:=office(map[string]string{"word/document.xml":`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Buyer</w:t></w:r><w:r><w:t> PO REP3</w:t></w:r></w:p></w:body></w:document>`})
 r:=Process(context.Background(),"order.docx",raw)
 if r.Format!="DOCX"||len(r.Segments)!=1||r.Segments[0].Content!="Buyer PO REP3" {t.Fatalf("%+v",r)}
}
func TestXLSXSharedTextCellAndFormula(t *testing.T){
 raw:=office(map[string]string{
 "xl/workbook.xml":`<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="PO Matrix" r:id="rId1"/></sheets></workbook>`,
 "xl/_rels/workbook.xml.rels":`<Relationships><Relationship Id="rId1" Target="worksheets/sheet1.xml"/></Relationships>`,
 "xl/sharedStrings.xml":`<sst><si><t>REP3</t></si><si><t>Quantity</t></si></sst>`,
 "xl/worksheets/sheet1.xml":`<worksheet><sheetData><row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c></row><row r="2"><c r="A2"><f>200*3</f><v>600</v></c></row></sheetData></worksheet>`,
 })
 r:=Process(context.Background(),"data.xlsx",raw)
 if r.Status!="EXTRACTED"||len(r.Segments)!=2||!strings.Contains(r.Segments[0].Content,"REP3")||!strings.Contains(r.Segments[1].Content,"=200*3 [cached: 600]")||!strings.Contains(string(r.Segments[1].Locator),"PO Matrix") {t.Fatalf("%+v",r)}
}
func TestUnsupportedOpaqueAndImages(t *testing.T){
 xls:=[]byte{0xd0,0xcf,0x11,0xe0,0xa1,0xb1,0x1a,0xe1}
 r:=Process(context.Background(),"legacy.xls",xls)
 if r.Status!="UNSUPPORTED"||r.Format!="XLS_LEGACY" {t.Fatalf("%+v",r)}
 r=Process(context.Background(),"binary.exe",[]byte{0,1,2,3})
 if r.Status!="UNSUPPORTED" {t.Fatalf("%+v",r)}
}
func TestNotTrustingZipExtension(t *testing.T){
 data:=office(map[string]string{"payload.txt":"untrusted generic ZIP"})
 format,_:=Detect("looks_like_an_order.xlsx",data)
 if format!="UNKNOWN" {t.Fatalf("format=%s",format)}
}
func TestSizeBudget(t *testing.T){
 raw:=make([]byte,MaxBytes+1)
 if r:=Process(context.Background(),"big.csv",raw);r.Status!="ERROR"{t.Fatalf("%+v",r)}
}
