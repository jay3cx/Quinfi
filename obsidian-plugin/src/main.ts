import { App, Modal, Notice, Plugin } from "obsidian";
import { FundAnalysisView, VIEW_TYPE_FUND_ANALYSIS } from "./view";

export default class FundMindPlugin extends Plugin {
  async onload(): Promise<void> {
    console.log("FundMind 插件加载中...");

    // 注册侧边栏视图
    this.registerView(VIEW_TYPE_FUND_ANALYSIS, (leaf) => new FundAnalysisView(leaf));

    // 注册命令：查询基金
    this.addCommand({
      id: "query-fund",
      name: "查询基金",
      callback: () => {
        new FundCodeModal(this.app, async (code) => {
          await this.openFundAnalysis(code);
        }).open();
      },
    });

    // 注册命令：打开分析面板
    this.addCommand({
      id: "open-analysis-panel",
      name: "打开分析面板",
      callback: () => {
        this.activateView();
      },
    });

    console.log("FundMind 插件加载完成");
  }

  async onunload(): Promise<void> {
    console.log("FundMind 插件卸载");
    this.app.workspace.detachLeavesOfType(VIEW_TYPE_FUND_ANALYSIS);
  }

  async activateView(): Promise<void> {
    const { workspace } = this.app;

    let leaf = workspace.getLeavesOfType(VIEW_TYPE_FUND_ANALYSIS)[0];
    if (!leaf) {
      const rightLeaf = workspace.getRightLeaf(false);
      if (rightLeaf) {
        leaf = rightLeaf;
        await leaf.setViewState({
          type: VIEW_TYPE_FUND_ANALYSIS,
          active: true,
        });
      }
    }

    if (leaf) {
      workspace.revealLeaf(leaf);
    }
  }

  async openFundAnalysis(code: string): Promise<void> {
    await this.activateView();

    const leaves = this.app.workspace.getLeavesOfType(VIEW_TYPE_FUND_ANALYSIS);
    if (leaves.length > 0) {
      const view = leaves[0].view as FundAnalysisView;
      await view.setFundCode(code);
      new Notice(`正在分析基金 ${code}...`);
    }
  }
}

// 基金代码输入弹窗
class FundCodeModal extends Modal {
  private onSubmit: (code: string) => void;
  private inputEl: HTMLInputElement | null = null;

  constructor(app: App, onSubmit: (code: string) => void) {
    super(app);
    this.onSubmit = onSubmit;
  }

  onOpen(): void {
    const { contentEl } = this;
    contentEl.empty();

    contentEl.createEl("h2", { text: "输入基金代码" });

    const inputContainer = contentEl.createEl("div", { cls: "fundmind-input-container" });
    this.inputEl = inputContainer.createEl("input", {
      type: "text",
      placeholder: "请输入6位基金代码，如 005827",
      cls: "fundmind-input",
    });
    this.inputEl.focus();

    // 回车提交
    this.inputEl.addEventListener("keydown", (e) => {
      if (e.key === "Enter") {
        this.submit();
      }
    });

    // 提交按钮
    const btnContainer = contentEl.createEl("div", { cls: "fundmind-btn-container" });
    const submitBtn = btnContainer.createEl("button", { text: "查询", cls: "mod-cta" });
    submitBtn.onclick = () => this.submit();

    const cancelBtn = btnContainer.createEl("button", { text: "取消" });
    cancelBtn.onclick = () => this.close();
  }

  private submit(): void {
    const code = this.inputEl?.value.trim();
    if (code && /^\d{6}$/.test(code)) {
      this.onSubmit(code);
      this.close();
    } else {
      new Notice("请输入有效的6位基金代码");
    }
  }

  onClose(): void {
    const { contentEl } = this;
    contentEl.empty();
  }
}
