import { Loader2, CheckCircle2 } from "lucide-react"

const TOOL_NAMES: Record<string, string> = {
    get_fund_info: "查询基金信息",
    get_nav_history: "查询净值走势",
    get_fund_holdings: "查询持仓数据",
    search_news: "搜索新闻资讯",
    search_funds: "搜索全市场基金",
    get_fund_ranking: "获取基金排行榜",
    get_portfolio: "读取持仓组合",
    run_debate: "发起多空辩论",
    detect_rebalance: "检测调仓变动",
}

interface ToolCallCardProps {
    toolName: string
    status: "loading" | "done"
}

export function ToolCallCard({ toolName, status }: ToolCallCardProps) {
    const displayName = TOOL_NAMES[toolName] || toolName

    return (
        <div className="flex items-center gap-2 py-1.5 px-3 rounded-md bg-[var(--color-sidebar-bg)] text-sm text-[var(--color-text-secondary)] w-fit">
            {status === "loading" ? (
                <Loader2 className="w-3.5 h-3.5 animate-spin" />
            ) : (
                <CheckCircle2 className="w-3.5 h-3.5 text-[var(--color-up)]" />
            )}
            <span>{status === "loading" ? `正在${displayName}...` : `${displayName}完成`}</span>
        </div>
    )
}
