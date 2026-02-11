var __defProp = Object.defineProperty;
var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
var __getOwnPropNames = Object.getOwnPropertyNames;
var __hasOwnProp = Object.prototype.hasOwnProperty;
var __export = (target, all) => {
  for (var name in all)
    __defProp(target, name, { get: all[name], enumerable: true });
};
var __copyProps = (to, from, except, desc) => {
  if (from && typeof from === "object" || typeof from === "function") {
    for (let key of __getOwnPropNames(from))
      if (!__hasOwnProp.call(to, key) && key !== except)
        __defProp(to, key, { get: () => from[key], enumerable: !(desc = __getOwnPropDesc(from, key)) || desc.enumerable });
  }
  return to;
};
var __toCommonJS = (mod) => __copyProps(__defProp({}, "__esModule", { value: true }), mod);

// src/main.ts
var main_exports = {};
__export(main_exports, {
  default: () => FundMindPlugin
});
module.exports = __toCommonJS(main_exports);
var import_obsidian2 = require("obsidian");

// src/view.ts
var import_obsidian = require("obsidian");

// src/api.ts
var API_BASE_URL = "http://localhost:8080/api/v1";
async function getFundInfo(code) {
  const response = await fetch(`${API_BASE_URL}/fund/${code}`);
  if (!response.ok) {
    throw new Error(`\u83B7\u53D6\u57FA\u91D1\u4FE1\u606F\u5931\u8D25: ${response.statusText}`);
  }
  return response.json();
}
async function analyzeFund(code) {
  const response = await fetch(`${API_BASE_URL}/chat`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json"
    },
    body: JSON.stringify({
      message: `\u5206\u6790\u57FA\u91D1 ${code}`,
      stream: false
    })
  });
  if (!response.ok) {
    throw new Error(`\u5206\u6790\u57FA\u91D1\u5931\u8D25: ${response.statusText}`);
  }
  const data = await response.json();
  return data.response || data.content || "\u5206\u6790\u5B8C\u6210";
}

// src/view.ts
var VIEW_TYPE_FUND_ANALYSIS = "fund-analysis-view";
var FundAnalysisView = class extends import_obsidian.ItemView {
  constructor(leaf) {
    super(leaf);
    this.fundCode = "";
    this.fundInfo = null;
    this.analysis = "";
    this.loading = false;
  }
  getViewType() {
    return VIEW_TYPE_FUND_ANALYSIS;
  }
  getDisplayText() {
    return this.fundCode ? `\u57FA\u91D1\u5206\u6790: ${this.fundCode}` : "\u57FA\u91D1\u5206\u6790";
  }
  getIcon() {
    return "line-chart";
  }
  async setFundCode(code) {
    this.fundCode = code;
    await this.loadData();
  }
  async loadData() {
    if (!this.fundCode) return;
    this.loading = true;
    this.render();
    try {
      const [fundInfo, analysis] = await Promise.all([
        getFundInfo(this.fundCode),
        analyzeFund(this.fundCode)
      ]);
      this.fundInfo = fundInfo;
      this.analysis = analysis;
    } catch (error) {
      this.analysis = `\u9519\u8BEF: ${error instanceof Error ? error.message : "\u672A\u77E5\u9519\u8BEF"}`;
    } finally {
      this.loading = false;
      this.render();
    }
  }
  async onOpen() {
    this.render();
  }
  async onClose() {
  }
  render() {
    const container = this.containerEl.children[1];
    container.empty();
    container.createEl("h2", { text: "\u{1F4CA} \u57FA\u91D1\u5206\u6790" });
    if (this.loading) {
      container.createEl("p", { text: "\u52A0\u8F7D\u4E2D...", cls: "fundmind-loading" });
      return;
    }
    if (!this.fundCode) {
      container.createEl("p", {
        text: '\u8BF7\u4F7F\u7528 "FundMind: \u67E5\u8BE2\u57FA\u91D1" \u547D\u4EE4\u8F93\u5165\u57FA\u91D1\u4EE3\u7801',
        cls: "fundmind-hint"
      });
      return;
    }
    const refreshBtn = container.createEl("button", { text: "\u{1F504} \u5237\u65B0" });
    refreshBtn.onclick = () => this.loadData();
    if (this.fundInfo) {
      const infoSection = container.createEl("div", { cls: "fundmind-section" });
      infoSection.createEl("h3", { text: "\u57FA\u672C\u4FE1\u606F" });
      const table = infoSection.createEl("table", { cls: "fundmind-table" });
      this.addTableRow(table, "\u4EE3\u7801", this.fundInfo.code);
      this.addTableRow(table, "\u540D\u79F0", this.fundInfo.name);
      this.addTableRow(table, "\u7C7B\u578B", this.fundInfo.type);
      this.addTableRow(table, "\u57FA\u91D1\u7ECF\u7406", this.fundInfo.manager);
      this.addTableRow(table, "\u57FA\u91D1\u516C\u53F8", this.fundInfo.company);
      this.addTableRow(table, "\u89C4\u6A21", `${this.fundInfo.scale}\u4EBF`);
      this.addTableRow(table, "\u6700\u65B0\u51C0\u503C", `${this.fundInfo.nav} (${this.fundInfo.navDate})`);
      const perfSection = container.createEl("div", { cls: "fundmind-section" });
      perfSection.createEl("h3", { text: "\u6536\u76CA\u8868\u73B0" });
      const perfTable = perfSection.createEl("table", { cls: "fundmind-table" });
      this.addGrowthRow(perfTable, "\u65E5\u6DA8\u5E45", this.fundInfo.dayGrowth);
      this.addGrowthRow(perfTable, "\u5468\u6DA8\u5E45", this.fundInfo.weekGrowth);
      this.addGrowthRow(perfTable, "\u6708\u6DA8\u5E45", this.fundInfo.monthGrowth);
      this.addGrowthRow(perfTable, "\u4E09\u6708\u6DA8\u5E45", this.fundInfo.threeMonthGrowth);
      this.addGrowthRow(perfTable, "\u516D\u6708\u6DA8\u5E45", this.fundInfo.sixMonthGrowth);
      this.addGrowthRow(perfTable, "\u5E74\u6DA8\u5E45", this.fundInfo.yearGrowth);
    }
    if (this.analysis) {
      const analysisSection = container.createEl("div", { cls: "fundmind-section" });
      analysisSection.createEl("h3", { text: "\u{1F916} AI \u5206\u6790" });
      analysisSection.createEl("div", {
        text: this.analysis,
        cls: "fundmind-analysis"
      });
    }
  }
  addTableRow(table, label, value) {
    const row = table.createEl("tr");
    row.createEl("td", { text: label, cls: "fundmind-label" });
    row.createEl("td", { text: value });
  }
  addGrowthRow(table, label, value) {
    const row = table.createEl("tr");
    row.createEl("td", { text: label, cls: "fundmind-label" });
    const valueCell = row.createEl("td");
    const cls = value >= 0 ? "fundmind-positive" : "fundmind-negative";
    valueCell.createEl("span", { text: `${value.toFixed(2)}%`, cls });
  }
};

// src/main.ts
var FundMindPlugin = class extends import_obsidian2.Plugin {
  async onload() {
    console.log("FundMind \u63D2\u4EF6\u52A0\u8F7D\u4E2D...");
    this.registerView(VIEW_TYPE_FUND_ANALYSIS, (leaf) => new FundAnalysisView(leaf));
    this.addCommand({
      id: "query-fund",
      name: "\u67E5\u8BE2\u57FA\u91D1",
      callback: () => {
        new FundCodeModal(this.app, async (code) => {
          await this.openFundAnalysis(code);
        }).open();
      }
    });
    this.addCommand({
      id: "open-analysis-panel",
      name: "\u6253\u5F00\u5206\u6790\u9762\u677F",
      callback: () => {
        this.activateView();
      }
    });
    console.log("FundMind \u63D2\u4EF6\u52A0\u8F7D\u5B8C\u6210");
  }
  async onunload() {
    console.log("FundMind \u63D2\u4EF6\u5378\u8F7D");
    this.app.workspace.detachLeavesOfType(VIEW_TYPE_FUND_ANALYSIS);
  }
  async activateView() {
    const { workspace } = this.app;
    let leaf = workspace.getLeavesOfType(VIEW_TYPE_FUND_ANALYSIS)[0];
    if (!leaf) {
      const rightLeaf = workspace.getRightLeaf(false);
      if (rightLeaf) {
        leaf = rightLeaf;
        await leaf.setViewState({
          type: VIEW_TYPE_FUND_ANALYSIS,
          active: true
        });
      }
    }
    if (leaf) {
      workspace.revealLeaf(leaf);
    }
  }
  async openFundAnalysis(code) {
    await this.activateView();
    const leaves = this.app.workspace.getLeavesOfType(VIEW_TYPE_FUND_ANALYSIS);
    if (leaves.length > 0) {
      const view = leaves[0].view;
      await view.setFundCode(code);
      new import_obsidian2.Notice(`\u6B63\u5728\u5206\u6790\u57FA\u91D1 ${code}...`);
    }
  }
};
var FundCodeModal = class extends import_obsidian2.Modal {
  constructor(app, onSubmit) {
    super(app);
    this.inputEl = null;
    this.onSubmit = onSubmit;
  }
  onOpen() {
    const { contentEl } = this;
    contentEl.empty();
    contentEl.createEl("h2", { text: "\u8F93\u5165\u57FA\u91D1\u4EE3\u7801" });
    const inputContainer = contentEl.createEl("div", { cls: "fundmind-input-container" });
    this.inputEl = inputContainer.createEl("input", {
      type: "text",
      placeholder: "\u8BF7\u8F93\u51656\u4F4D\u57FA\u91D1\u4EE3\u7801\uFF0C\u5982 005827",
      cls: "fundmind-input"
    });
    this.inputEl.focus();
    this.inputEl.addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        this.submit();
      }
    });
    const btnContainer = contentEl.createEl("div", { cls: "fundmind-btn-container" });
    const submitBtn = btnContainer.createEl("button", { text: "\u67E5\u8BE2", cls: "mod-cta" });
    submitBtn.onclick = () => this.submit();
    const cancelBtn = btnContainer.createEl("button", { text: "\u53D6\u6D88" });
    cancelBtn.onclick = () => this.close();
  }
  submit() {
    var _a;
    const code = (_a = this.inputEl) == null ? void 0 : _a.value.trim();
    if (code && /^\d{6}$/.test(code)) {
      this.onSubmit(code);
      this.close();
    } else {
      new import_obsidian2.Notice("\u8BF7\u8F93\u5165\u6709\u6548\u76846\u4F4D\u57FA\u91D1\u4EE3\u7801");
    }
  }
  onClose() {
    const { contentEl } = this;
    contentEl.empty();
  }
};
