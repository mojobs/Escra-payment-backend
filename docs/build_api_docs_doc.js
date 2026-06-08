const fs = require("node:fs");
const path = require("node:path");
const { execFileSync } = require("node:child_process");

const workspaceRoot = path.resolve(__dirname, "..");
const outputPath = path.join(__dirname, "escra-api-docs.docx");
const buildRoot = path.join(workspaceRoot, ".docx-build");
const packageRoot = path.join(buildRoot, "escra-api-docs");
const zipPath = path.join(buildRoot, "escra-api-docs.zip");
const verifyRoot = path.join(buildRoot, "verify-api-docs");

const sections = [
  {
    title: "Health",
    rows: [
      ["GET", "/health", "Checks if the backend server is running."],
    ],
  },
  {
    title: "Authentication",
    rows: [
      ["POST", "/api/v1/auth/register", "Creates a new ESCRA user account."],
      ["POST", "/api/v1/auth/login", "Logs a user in and returns authentication tokens."],
    ],
  },
  {
    title: "User",
    rows: [
      ["GET", "/api/v1/users/profile", "Gets the logged-in user's profile, metrics, and merchant details."],
      ["PUT", "/api/v1/users/profile/business", "Updates merchant business details for seller profile screens."],
      ["POST", "/api/v1/users/kyc/upload", "Uploads a KYC document and marks the user as under review."],
    ],
  },
  {
    title: "Wallets",
    rows: [
      ["GET", "/api/v1/wallets/balance", "Gets the user's wallet balance."],
      ["POST", "/api/v1/wallets/checkout/kora", "Creates a Kora checkout link for direct wallet funding."],
      ["GET", "/api/v1/wallets/", "Gets the user's wallet details."],
    ],
  },
  {
    title: "Transactions",
    rows: [
      ["POST", "/api/v1/transactions/transfer", "Transfers money between ESCRA wallets."],
      ["GET", "/api/v1/transactions/history", "Gets the user's transaction history."],
      ["GET", "/api/v1/transactions/:reference", "Gets one transaction by its reference."],
    ],
  },
  {
    title: "Escrow",
    rows: [
      ["POST", "/api/v1/escrows/orders", "Seller creates a new escrow order."],
      ["GET", "/api/v1/escrows/orders", "Lists the user's escrow orders."],
      ["GET", "/api/v1/escrows/orders/:reference", "Gets one escrow order."],
      ["GET", "/api/v1/public/escrows/orders/:reference", "Gets the public order preview for buyers."],
      ["POST", "/api/v1/escrows/orders/:reference/checkout/kora", "Starts Kora checkout for buyer payment."],
      ["POST", "/api/v1/escrows/orders/:reference/fund", "Manually funds escrow from the buyer wallet."],
      ["POST", "/api/v1/escrows/orders/:reference/ship", "Seller marks the item as shipped."],
      ["POST", "/api/v1/escrows/orders/:reference/mark-delivered", "Seller marks the order as delivered."],
      ["POST", "/api/v1/escrows/orders/:reference/confirm-delivery", "Buyer confirms delivery."],
      ["POST", "/api/v1/escrows/orders/:reference/release", "Releases escrow funds to seller if eligible."],
      ["POST", "/api/v1/escrows/orders/:reference/dispute", "Opens a dispute on an escrow order."],
      ["POST", "/api/v1/escrows/orders/:reference/cancel", "Cancels an escrow order."],
      ["POST", "/api/v1/escrows/orders/:reference/refund", "Refunds an escrow order."],
    ],
  },
  {
    title: "Admin",
    rows: [
      ["POST", "/api/v1/admin/escrows/orders/:reference/resolve-dispute", "Admin resolves a dispute by releasing or refunding funds."],
      ["POST", "/api/v1/admin/providers/kora/refunds", "Admin initiates a Kora refund."],
      ["POST", "/api/v1/admin/users/:userId/verify", "Admin approves or rejects a user's KYC verification."],
    ],
  },
  {
    title: "Kora Provider",
    rows: [
      ["POST", "/api/v1/providers/kora/kyc/verify", "Verifies user KYC with Kora."],
      ["GET", "/api/v1/providers/kora/banks", "Lists Kora-supported banks."],
      ["GET", "/api/v1/providers/kora/banks/resolve", "Resolves a bank account name."],
      ["GET", "/api/v1/providers/kora/balances", "Gets Kora balances for reconciliation."],
      ["POST", "/api/v1/providers/kora/virtual-accounts", "Creates a Kora virtual bank account for pay-ins."],
      ["GET", "/api/v1/providers/kora/virtual-accounts", "Lists the user's Kora virtual accounts."],
      ["POST", "/api/v1/providers/kora/payouts/bank", "Sends money to a seller's bank account."],
    ],
  },
  {
    title: "Quidax Provider",
    rows: [
      ["POST", "/api/v1/providers/quidax/withdrawals", "Initiates a crypto withdrawal or transfer through Quidax."],
    ],
  },
  {
    title: "Webhooks",
    rows: [
      ["POST", "/api/v1/webhooks/kora", "Receives Kora payment, checkout, payout, and refund events."],
      ["POST", "/api/v1/webhooks/quidax", "Receives Quidax crypto transaction events."],
    ],
  },
  {
    title: "Limits",
    rows: [
      ["GET", "/api/v1/limits/", "Gets the logged-in user's transaction limits and usage."],
    ],
  },
];

function xmlEscape(value) {
  return String(value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&apos;");
}

function ensureDir(dirPath) {
  fs.mkdirSync(dirPath, { recursive: true });
}

function resetDir(dirPath) {
  fs.rmSync(dirPath, { recursive: true, force: true });
  ensureDir(dirPath);
}

function writeFile(relativePath, contents) {
  const fullPath = path.join(packageRoot, relativePath);
  ensureDir(path.dirname(fullPath));
  fs.writeFileSync(fullPath, contents, "utf8");
}

function powershellLiteral(filePath) {
  return filePath.replace(/'/g, "''");
}

function run(text, options = {}) {
  const props = [];
  if (options.bold) props.push("<w:b/>");
  if (options.monospace) props.push('<w:rFonts w:ascii="Consolas" w:hAnsi="Consolas"/>');
  if (options.size) {
    props.push(`<w:sz w:val="${options.size}"/>`);
    props.push(`<w:szCs w:val="${options.size}"/>`);
  }
  if (options.color) props.push(`<w:color w:val="${options.color}"/>`);
  const rPr = props.length ? `<w:rPr>${props.join("")}</w:rPr>` : "";
  return `<w:r>${rPr}<w:t xml:space="preserve">${xmlEscape(text)}</w:t></w:r>`;
}

function paragraph(text, style = "Normal", options = {}) {
  const pPr = [
    `<w:pStyle w:val="${style}"/>`,
    options.keepNext ? "<w:keepNext/>" : "",
  ].join("");
  return `<w:p><w:pPr>${pPr}</w:pPr>${run(text, options)}</w:p>`;
}

function tableCell(text, width, options = {}) {
  const fill = options.header ? '<w:shd w:val="clear" w:color="auto" w:fill="E8EEF5"/>' : "";
  const vAlign = '<w:vAlign w:val="center"/>';
  const pStyle = options.header ? "TableHeader" : options.method ? "TableMethod" : options.api ? "TableAPI" : "TableText";
  const textOptions = options.api ? { monospace: true, size: 18 } : {};
  if (options.header) {
    textOptions.bold = true;
    textOptions.color = "0B2545";
    textOptions.size = 19;
  }
  return `<w:tc>
    <w:tcPr>
      <w:tcW w:w="${width}" w:type="dxa"/>
      ${fill}
      ${vAlign}
      <w:tcMar>
        <w:top w:w="80" w:type="dxa"/>
        <w:bottom w:w="80" w:type="dxa"/>
        <w:start w:w="120" w:type="dxa"/>
        <w:end w:w="120" w:type="dxa"/>
      </w:tcMar>
    </w:tcPr>
    ${paragraph(text, pStyle, textOptions)}
  </w:tc>`;
}

function tableRow(cells, options = {}) {
  const widths = [980, 4380, 4000];
  const headerPr = options.header ? "<w:tblHeader/>" : "";
  return `<w:tr><w:trPr>${headerPr}</w:trPr>${cells.map((cell, index) => tableCell(cell, widths[index], {
    header: options.header,
    method: index === 0,
    api: index === 1,
  })).join("")}</w:tr>`;
}

function table(rows) {
  const header = tableRow(["Method", "API", "What It Does"], { header: true });
  const body = rows.map((row) => tableRow(row)).join("");
  return `<w:tbl>
    <w:tblPr>
      <w:tblStyle w:val="TableGrid"/>
      <w:tblW w:w="9360" w:type="dxa"/>
      <w:tblInd w:w="120" w:type="dxa"/>
      <w:tblLayout w:type="fixed"/>
      <w:tblLook w:firstRow="1" w:lastRow="0" w:firstColumn="0" w:lastColumn="0" w:noHBand="1" w:noVBand="1"/>
      <w:tblBorders>
        <w:top w:val="single" w:sz="4" w:space="0" w:color="B7C2D0"/>
        <w:left w:val="single" w:sz="4" w:space="0" w:color="B7C2D0"/>
        <w:bottom w:val="single" w:sz="4" w:space="0" w:color="B7C2D0"/>
        <w:right w:val="single" w:sz="4" w:space="0" w:color="B7C2D0"/>
        <w:insideH w:val="single" w:sz="4" w:space="0" w:color="D7DEE8"/>
        <w:insideV w:val="single" w:sz="4" w:space="0" w:color="D7DEE8"/>
      </w:tblBorders>
    </w:tblPr>
    <w:tblGrid>
      <w:gridCol w:w="980"/>
      <w:gridCol w:w="4380"/>
      <w:gridCol w:w="4000"/>
    </w:tblGrid>
    ${header}${body}
  </w:tbl>`;
}

function buildBodyXml() {
  const parts = [
    paragraph("ESCRA API Docs", "Title"),
    paragraph("APIs and what they do", "Subtitle"),
    paragraph("Base URL: http://localhost:8080", "Meta"),
    paragraph("API prefix: /api/v1", "Meta"),
    paragraph("Protected routes require Authorization: Bearer <token>. Admin routes require X-Admin-Key. Payment and state-changing POST routes should use Idempotency-Key.", "Normal"),
  ];

  for (const section of sections) {
    parts.push(paragraph(section.title, "Heading1", { keepNext: true }));
    parts.push(table(section.rows));
    parts.push(paragraph("", "TableSpacer"));
  }

  return parts.join("\n");
}

function buildContentTypesXml() {
  return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
  <Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
  <Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>
  <Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>
</Types>`;
}

function buildRootRelsXml() {
  return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
  <Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>
</Relationships>`;
}

function buildDocumentRelsXml() {
  return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`;
}

function buildCoreXml() {
  const created = new Date().toISOString();
  return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <dc:title>ESCRA API Docs</dc:title>
  <dc:creator>Codex</dc:creator>
  <cp:lastModifiedBy>Codex</cp:lastModifiedBy>
  <dcterms:created xsi:type="dcterms:W3CDTF">${created}</dcterms:created>
  <dcterms:modified xsi:type="dcterms:W3CDTF">${created}</dcterms:modified>
</cp:coreProperties>`;
}

function buildAppXml() {
  return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">
  <Application>Codex</Application>
</Properties>`;
}

function buildStylesXml() {
  return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:docDefaults>
    <w:rPrDefault>
      <w:rPr>
        <w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/>
        <w:sz w:val="22"/>
        <w:szCs w:val="22"/>
      </w:rPr>
    </w:rPrDefault>
    <w:pPrDefault>
      <w:pPr>
        <w:spacing w:after="120" w:line="300" w:lineRule="auto"/>
      </w:pPr>
    </w:pPrDefault>
  </w:docDefaults>
  <w:style w:type="paragraph" w:default="1" w:styleId="Normal">
    <w:name w:val="Normal"/>
    <w:qFormat/>
    <w:pPr><w:spacing w:before="0" w:after="120" w:line="300" w:lineRule="auto"/></w:pPr>
    <w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/><w:sz w:val="22"/><w:szCs w:val="22"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Title">
    <w:name w:val="Title"/>
    <w:basedOn w:val="Normal"/>
    <w:qFormat/>
    <w:pPr><w:spacing w:before="0" w:after="140"/><w:keepNext/></w:pPr>
    <w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/><w:b/><w:sz w:val="40"/><w:szCs w:val="40"/><w:color w:val="0B2545"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Subtitle">
    <w:name w:val="Subtitle"/>
    <w:basedOn w:val="Normal"/>
    <w:qFormat/>
    <w:pPr><w:spacing w:before="0" w:after="80"/></w:pPr>
    <w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/><w:sz w:val="24"/><w:szCs w:val="24"/><w:color w:val="555555"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Meta">
    <w:name w:val="Meta"/>
    <w:basedOn w:val="Normal"/>
    <w:pPr><w:spacing w:before="0" w:after="60"/></w:pPr>
    <w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/><w:b/><w:sz w:val="20"/><w:szCs w:val="20"/><w:color w:val="1F4D78"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading1">
    <w:name w:val="heading 1"/>
    <w:basedOn w:val="Normal"/>
    <w:qFormat/>
    <w:pPr><w:spacing w:before="360" w:after="200"/><w:keepNext/><w:outlineLvl w:val="0"/></w:pPr>
    <w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/><w:b/><w:sz w:val="32"/><w:szCs w:val="32"/><w:color w:val="2E74B5"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="TableHeader">
    <w:name w:val="Table Header"/>
    <w:basedOn w:val="Normal"/>
    <w:pPr><w:spacing w:before="0" w:after="0" w:line="260" w:lineRule="auto"/></w:pPr>
    <w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/><w:b/><w:sz w:val="19"/><w:szCs w:val="19"/><w:color w:val="0B2545"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="TableMethod">
    <w:name w:val="Table Method"/>
    <w:basedOn w:val="Normal"/>
    <w:pPr><w:jc w:val="center"/><w:spacing w:before="0" w:after="0" w:line="260" w:lineRule="auto"/></w:pPr>
    <w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/><w:b/><w:sz w:val="19"/><w:szCs w:val="19"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="TableAPI">
    <w:name w:val="Table API"/>
    <w:basedOn w:val="Normal"/>
    <w:pPr><w:spacing w:before="0" w:after="0" w:line="260" w:lineRule="auto"/></w:pPr>
    <w:rPr><w:rFonts w:ascii="Consolas" w:hAnsi="Consolas"/><w:sz w:val="18"/><w:szCs w:val="18"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="TableText">
    <w:name w:val="Table Text"/>
    <w:basedOn w:val="Normal"/>
    <w:pPr><w:spacing w:before="0" w:after="0" w:line="260" w:lineRule="auto"/></w:pPr>
    <w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/><w:sz w:val="20"/><w:szCs w:val="20"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="TableSpacer">
    <w:name w:val="Table Spacer"/>
    <w:basedOn w:val="Normal"/>
    <w:pPr><w:spacing w:before="0" w:after="40"/></w:pPr>
    <w:rPr><w:sz w:val="2"/><w:szCs w:val="2"/></w:rPr>
  </w:style>
</w:styles>`;
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
</w:document>`;
}

function buildDocx() {
  resetDir(packageRoot);
  fs.rmSync(zipPath, { force: true });
  fs.rmSync(outputPath, { force: true });
  fs.rmSync(verifyRoot, { recursive: true, force: true });
  ensureDir(buildRoot);

  writeFile("[Content_Types].xml", buildContentTypesXml());
  writeFile("_rels/.rels", buildRootRelsXml());
  writeFile("docProps/core.xml", buildCoreXml());
  writeFile("docProps/app.xml", buildAppXml());
  writeFile("word/document.xml", buildDocumentXml(buildBodyXml()));
  writeFile("word/styles.xml", buildStylesXml());
  writeFile("word/_rels/document.xml.rels", buildDocumentRelsXml());

  const sourceForZip = powershellLiteral(path.join(packageRoot, "*"));
  const zipLiteral = powershellLiteral(zipPath);
  const docxLiteral = powershellLiteral(outputPath);
  const verifyLiteral = powershellLiteral(verifyRoot);
  const command = [
    "$ErrorActionPreference = 'Stop'",
    `Compress-Archive -Path '${sourceForZip}' -DestinationPath '${zipLiteral}' -Force`,
    `Copy-Item -LiteralPath '${zipLiteral}' -Destination '${docxLiteral}' -Force`,
    `Expand-Archive -LiteralPath '${zipLiteral}' -DestinationPath '${verifyLiteral}' -Force`,
  ].join("; ");

  execFileSync("powershell.exe", ["-NoLogo", "-NonInteractive", "-Command", command], {
    stdio: "inherit",
  });

  console.log(`Built ${outputPath}`);
}

buildDocx();
