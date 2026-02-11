import { ItemView, WorkspaceLeaf } from "obsidian";
import { FundInfo, getFundInfo, analyzeFund } from "./api";

export const VIEW_TYPE_FUND_ANALYSIS = "fund-analysis-view";

export class FundAnalysisView extends ItemView {
  private fundCode: string = "";
  private fundInfo: FundInfo | null = null;
  private analysis: string = "";
  private loading: boolean = false;

  constructor(leaf: WorkspaceLeaf) {
    super(leaf);
  }

  getViewType(): string {
    return VIEW_TYPE_FUND_ANALYSIS;
  }

  getDisplayText(): string {
    return this.fundCode ? `基金分析: ${this.fundCode}` : "基金分析";
  }

  getIcon(): string {
    return "line-chart";
  }

  async setFundCode(code: string): Promise<void> {
    this.fundCode = code;
    await this.loadData();
  }

  async loadData(): Promise<void> {
    if (!this.fundCode) return;

    this.loading = true;
    this.render();

    try {
      // 并行获取基金信息和分析
      const [fundInfo, analysis] = await Promise.all([
        getFundInfo(this.fundCode),
        analyzeFund(this.fundCode),
      ]);
      this.fundInfo = fundInfo;
      this.analysis = analysis;
    } catch (error) {
      this.analysis = `错误: ${error instanceof Error ? error.message : "未知错误"}`;
    } finally {
      this.loading = false;
      this.render();
    }
  }

  async onOpen(): Promise<void> {
    this.render();
  }

  async onClose(): Promise<void> {
    // 清理
  }

  private render(): void {
    const container = this.containerEl.children[1];
    container.empty();

    // 标题
    container.createEl("h2", { text: "📊 基金分析" });

    if (this.loading) {
      container.createEl("p", { text: "加载中...", cls: "fundmind-loading" });
      return;
    }

    if (!this.fundCode) {
      container.createEl("p", {
        text: '请使用 "FundMind: 查询基金" 命令输入基金代码',
        cls: "fundmind-hint",
      });
      return;
    }

    // 刷新按钮
    const refreshBtn = container.createEl("button", { text: "🔄 刷新" });
    refreshBtn.onclick = () => this.loadData();

    // 基金基本信息
    if (this.fundInfo) {
      const infoSection = container.createEl("div", { cls: "fundmind-section" });
      infoSection.createEl("h3", { text: "基本信息" });

      const table = infoSection.createEl("table", { cls: "fundmind-table" });
      this.addTableRow(table, "代码", this.fundInfo.code);
      this.addTableRow(table, "名称", this.fundInfo.name);
      this.addTableRow(table, "类型", this.fundInfo.type);
      this.addTableRow(table, "基金经理", this.fundInfo.manager);
      this.addTableRow(table, "基金公司", this.fundInfo.company);
      this.addTableRow(table, "规模", `${this.fundInfo.scale}亿`);
      this.addTableRow(table, "最新净值", `${this.fundInfo.nav} (${this.fundInfo.navDate})`);

      // 收益率
      const perfSection = container.createEl("div", { cls: "fundmind-section" });
      perfSection.createEl("h3", { text: "收益表现" });
      const perfTable = perfSection.createEl("table", { cls: "fundmind-table" });
      this.addGrowthRow(perfTable, "日涨幅", this.fundInfo.dayGrowth);
      this.addGrowthRow(perfTable, "周涨幅", this.fundInfo.weekGrowth);
      this.addGrowthRow(perfTable, "月涨幅", this.fundInfo.monthGrowth);
      this.addGrowthRow(perfTable, "三月涨幅", this.fundInfo.threeMonthGrowth);
      this.addGrowthRow(perfTable, "六月涨幅", this.fundInfo.sixMonthGrowth);
      this.addGrowthRow(perfTable, "年涨幅", this.fundInfo.yearGrowth);
    }

    // AI 分析
    if (this.analysis) {
      const analysisSection = container.createEl("div", { cls: "fundmind-section" });
      analysisSection.createEl("h3", { text: "🤖 AI 分析" });
      analysisSection.createEl("div", {
        text: this.analysis,
        cls: "fundmind-analysis",
      });
    }
  }

  private addTableRow(table: HTMLTableElement, label: string, value: string): void {
    const row = table.createEl("tr");
    row.createEl("td", { text: label, cls: "fundmind-label" });
    row.createEl("td", { text: value });
  }

  private addGrowthRow(table: HTMLTableElement, label: string, value: number): void {
    const row = table.createEl("tr");
    row.createEl("td", { text: label, cls: "fundmind-label" });
    const valueCell = row.createEl("td");
    const cls = value >= 0 ? "fundmind-positive" : "fundmind-negative";
    valueCell.createEl("span", { text: `${value.toFixed(2)}%`, cls });
  }
}
