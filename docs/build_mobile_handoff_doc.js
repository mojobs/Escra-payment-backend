const fs = require("node:fs");
const path = require("node:path");
const { execFileSync } = require("node:child_process");

const workspaceRoot = path.resolve(__dirname, "..");
const sourcePath = path.join(__dirname, "escra-mobile-developer-handoff.md");
const outputPath = path.join(__dirname, "escra-mobile-developer-handoff.docx");
const buildRoot = path.join(workspaceRoot, ".docx-build");
const packageRoot = path.join(buildRoot, "escra-mobile-developer-handoff");
const zipPath = path.join(buildRoot, "escra-mobile-developer-handoff.zip");
const verifyRoot = path.join(buildRoot, "verify-mobile-handoff");

function xmlEscape(value) {
  return String(value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&apos;");
}

function toWText(text) {
  return `<w:r><w:t xml:space="preserve">${xmlEscape(text)}</w:t></w:r>`;
}

function paragraph({ text, style = "Normal", numId = null }) {
  let pPr = `<w:pStyle w:val="${style}"/>`;
  if (numId !== null) {
    pPr += `<w:numPr><w:ilvl w:val="0"/><w:numId w:val="${numId}"/></w:numPr>`;
  }

  return `<w:p><w:pPr>${pPr}</w:pPr>${toWText(text)}</w:p>`;
}

function parseMarkdown(markdown) {
  const lines = markdown.replace(/\r\n/g, "\n").split("\n");
  const nodes = [];

  for (const line of lines) {
    const trimmed = line.trimEnd();
    if (trimmed.trim() === "") {
      continue;
    }

    if (trimmed.startsWith("# ")) {
      nodes.push(paragraph({ text: trimmed.slice(2), style: "Title" }));
      continue;
    }
    if (trimmed.startsWith("## ")) {
      nodes.push(paragraph({ text: trimmed.slice(3), style: "Heading1" }));
      continue;
    }
    if (trimmed.startsWith("### ")) {
      nodes.push(paragraph({ text: trimmed.slice(4), style: "Heading2" }));
      continue;
    }
    if (trimmed.startsWith("#### ")) {
      nodes.push(paragraph({ text: trimmed.slice(5), style: "Heading3" }));
      continue;
    }
    if (trimmed.startsWith("- ")) {
      nodes.push(paragraph({ text: trimmed.slice(2), style: "ListParagraph", numId: 1 }));
      continue;
    }
    if (/^\d+\.\s+/.test(trimmed)) {
      nodes.push(paragraph({ text: trimmed.replace(/^\d+\.\s+/, ""), style: "ListParagraph", numId: 2 }));
      continue;
    }
    if (/^(Version|Audience|Product|Owner):/.test(trimmed)) {
      nodes.push(paragraph({ text: trimmed, style: "Subtitle" }));
      continue;
    }
    nodes.push(paragraph({ text: trimmed, style: "Normal" }));
  }

  return nodes.join("");
}

function ensureDir(dirPath) {
  fs.mkdirSync(dirPath, { recursive: true });
}

function writeFile(relativePath, contents) {
  const fullPath = path.join(packageRoot, relativePath);
  ensureDir(path.dirname(fullPath));
  fs.writeFileSync(fullPath, contents, "utf8");
}

function powershellLiteral(filePath) {
  return filePath.replace(/'/g, "''");
}

function buildContentTypesXml() {
  return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
  <Override PartName="/word/numbering.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"/>
  <Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>
  <Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>
</Types>
`;
}

function buildRootRelsXml() {
  return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
</Relationships>
`;
}

function buildDocumentRelsXml() {
  return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>
</Relationships>
`;
}

function buildCoreXml() {
  const created = new Date().toISOString();
  return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <dc:title>ESCRA Mobile Developer Handoff</dc:title>
  <dc:creator>Codex</dc:creator>
  <cp:lastModifiedBy>Codex</cp:lastModifiedBy>
  <dcterms:created xsi:type="dcterms:W3CDTF">${created}</dcterms:created>
  <dcterms:modified xsi:type="dcterms:W3CDTF">${created}</dcterms:modified>
</cp:coreProperties>
`;
}

function buildAppXml() {
  return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">
  <Application>Codex</Application>
</Properties>
`;
}

function buildStylesXml() {
  return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:docDefaults>
    <w:rPrDefault>
      <w:rPr>
        <w:rFonts w:ascii="Arial" w:hAnsi="Arial"/>
        <w:sz w:val="22"/>
        <w:szCs w:val="22"/>
        <w:color w:val="000000"/>
      </w:rPr>
    </w:rPrDefault>
    <w:pPrDefault>
      <w:pPr>
        <w:spacing w:after="160" w:line="276" w:lineRule="auto"/>
      </w:pPr>
    </w:pPrDefault>
  </w:docDefaults>
  <w:style w:type="paragraph" w:default="1" w:styleId="Normal">
    <w:name w:val="Normal"/>
    <w:qFormat/>
    <w:rPr>
      <w:rFonts w:ascii="Arial" w:hAnsi="Arial"/>
      <w:sz w:val="22"/>
      <w:szCs w:val="22"/>
    </w:rPr>
    <w:pPr>
      <w:spacing w:after="160" w:line="276" w:lineRule="auto"/>
    </w:pPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Title">
    <w:name w:val="Title"/>
    <w:basedOn w:val="Normal"/>
    <w:qFormat/>
    <w:pPr>
      <w:spacing w:before="0" w:after="120"/>
      <w:keepNext/>
    </w:pPr>
    <w:rPr>
      <w:rFonts w:ascii="Arial" w:hAnsi="Arial"/>
      <w:b/>
      <w:sz w:val="36"/>
      <w:szCs w:val="36"/>
      <w:color w:val="000000"/>
    </w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Subtitle">
    <w:name w:val="Subtitle"/>
    <w:basedOn w:val="Normal"/>
    <w:qFormat/>
    <w:pPr>
      <w:spacing w:before="0" w:after="60"/>
    </w:pPr>
    <w:rPr>
      <w:rFonts w:ascii="Arial" w:hAnsi="Arial"/>
      <w:sz w:val="22"/>
      <w:szCs w:val="22"/>
      <w:color w:val="555555"/>
    </w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading1">
    <w:name w:val="heading 1"/>
    <w:basedOn w:val="Normal"/>
    <w:qFormat/>
    <w:pPr>
      <w:spacing w:before="320" w:after="120"/>
      <w:keepNext/>
      <w:outlineLvl w:val="0"/>
    </w:pPr>
    <w:rPr>
      <w:rFonts w:ascii="Arial" w:hAnsi="Arial"/>
      <w:b/>
      <w:sz w:val="28"/>
      <w:szCs w:val="28"/>
      <w:color w:val="000000"/>
    </w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading2">
    <w:name w:val="heading 2"/>
    <w:basedOn w:val="Normal"/>
    <w:qFormat/>
    <w:pPr>
      <w:spacing w:before="260" w:after="100"/>
      <w:keepNext/>
      <w:outlineLvl w:val="1"/>
    </w:pPr>
    <w:rPr>
      <w:rFonts w:ascii="Arial" w:hAnsi="Arial"/>
      <w:b/>
      <w:sz w:val="24"/>
      <w:szCs w:val="24"/>
      <w:color w:val="000000"/>
    </w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading3">
    <w:name w:val="heading 3"/>
    <w:basedOn w:val="Normal"/>
    <w:qFormat/>
    <w:pPr>
      <w:spacing w:before="220" w:after="80"/>
      <w:keepNext/>
      <w:outlineLvl w:val="2"/>
    </w:pPr>
    <w:rPr>
      <w:rFonts w:ascii="Arial" w:hAnsi="Arial"/>
      <w:b/>
      <w:sz w:val="22"/>
      <w:szCs w:val="22"/>
      <w:color w:val="434343"/>
    </w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="ListParagraph">
    <w:name w:val="List Paragraph"/>
    <w:basedOn w:val="Normal"/>
    <w:qFormat/>
    <w:pPr>
      <w:spacing w:after="80" w:line="276" w:lineRule="auto"/>
    </w:pPr>
    <w:rPr>
      <w:rFonts w:ascii="Arial" w:hAnsi="Arial"/>
      <w:sz w:val="22"/>
      <w:szCs w:val="22"/>
    </w:rPr>
  </w:style>
</w:styles>
`;
}

function buildNumberingXml() {
  return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:abstractNum w:abstractNumId="1">
    <w:multiLevelType w:val="hybridMultilevel"/>
    <w:lvl w:ilvl="0">
      <w:start w:val="1"/>
      <w:numFmt w:val="bullet"/>
      <w:lvlText w:val="•"/>
      <w:lvlJc w:val="left"/>
      <w:pPr>
        <w:tabs>
          <w:tab w:val="num" w:pos="720"/>
        </w:tabs>
        <w:ind w:left="720" w:hanging="360"/>
        <w:spacing w:after="80" w:line="276" w:lineRule="auto"/>
      </w:pPr>
      <w:rPr>
        <w:rFonts w:ascii="Arial" w:hAnsi="Arial"/>
      </w:rPr>
    </w:lvl>
  </w:abstractNum>
  <w:abstractNum w:abstractNumId="2">
    <w:multiLevelType w:val="hybridMultilevel"/>
    <w:lvl w:ilvl="0">
      <w:start w:val="1"/>
      <w:numFmt w:val="decimal"/>
      <w:lvlText w:val="%1."/>
      <w:lvlJc w:val="left"/>
      <w:pPr>
        <w:tabs>
          <w:tab w:val="num" w:pos="720"/>
        </w:tabs>
        <w:ind w:left="720" w:hanging="360"/>
        <w:spacing w:after="80" w:line="276" w:lineRule="auto"/>
      </w:pPr>
      <w:rPr>
        <w:rFonts w:ascii="Arial" w:hAnsi="Arial"/>
      </w:rPr>
    </w:lvl>
  </w:abstractNum>
  <w:num w:numId="1">
    <w:abstractNumId w:val="1"/>
  </w:num>
  <w:num w:numId="2">
    <w:abstractNumId w:val="2"/>
  </w:num>
</w:numbering>
`;
}

function buildDocumentXml(bodyXml) {
  return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:wpc="http://schemas.microsoft.com/office/word/2010/wordprocessingCanvas" xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" xmlns:o="urn:schemas-microsoft-com:office:office" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math" xmlns:v="urn:schemas-microsoft-com:vml" xmlns:wp14="http://schemas.microsoft.com/office/word/2010/wordprocessingDrawing" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" xmlns:w10="urn:schemas-microsoft-com:office:word" xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml" xmlns:wpg="http://schemas.microsoft.com/office/word/2010/wordprocessingGroup" xmlns:wpi="http://schemas.microsoft.com/office/word/2010/wordprocessingInk" xmlns:wne="http://schemas.microsoft.com/office/word/2006/wordml" xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape" mc:Ignorable="w14 wp14">
  <w:body>
    ${bodyXml}
    <w:sectPr>
      <w:pgSz w:w="12240" w:h="15840"/>
      <w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="708" w:footer="708" w:gutter="0"/>
      <w:cols w:space="708"/>
      <w:docGrid w:linePitch="360"/>
    </w:sectPr>
  </w:body>
</w:document>
`;
}

function resetDir(dirPath) {
  fs.rmSync(dirPath, { recursive: true, force: true });
  ensureDir(dirPath);
}

function main() {
  const markdown = fs.readFileSync(sourcePath, "utf8");
  const bodyXml = parseMarkdown(markdown);

  resetDir(packageRoot);
  fs.rmSync(zipPath, { force: true });
  fs.rmSync(outputPath, { force: true });
  fs.rmSync(verifyRoot, { recursive: true, force: true });
  ensureDir(buildRoot);

  writeFile("[Content_Types].xml", buildContentTypesXml());
  writeFile("_rels/.rels", buildRootRelsXml());
  writeFile("docProps/core.xml", buildCoreXml());
  writeFile("docProps/app.xml", buildAppXml());
  writeFile("word/document.xml", buildDocumentXml(bodyXml));
  writeFile("word/styles.xml", buildStylesXml());
  writeFile("word/numbering.xml", buildNumberingXml());
  writeFile("word/_rels/document.xml.rels", buildDocumentRelsXml());

  const sourceForZip = powershellLiteral(path.join(packageRoot, "*"));
  const zipLiteral = powershellLiteral(zipPath);
  const docxLiteral = powershellLiteral(outputPath);
  const verifyLiteral = powershellLiteral(verifyRoot);

  const command = [
    `$ErrorActionPreference = 'Stop'`,
    `Compress-Archive -Path '${sourceForZip}' -DestinationPath '${zipLiteral}' -Force`,
    `Copy-Item -LiteralPath '${zipLiteral}' -Destination '${docxLiteral}' -Force`,
    `Expand-Archive -LiteralPath '${zipLiteral}' -DestinationPath '${verifyLiteral}' -Force`,
  ].join("; ");

  execFileSync("powershell.exe", ["-NoLogo", "-NonInteractive", "-Command", command], {
    stdio: "inherit",
  });

  console.log(`Built ${outputPath}`);
}

main();
