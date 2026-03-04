import { useEffect, lazy, Suspense } from "react"
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom"
import { useSessionStore } from "@/stores/sessionStore"
import { Layout } from "@/components/Layout"
import ChatPage from "@/pages/ChatPage"
import MarketPage from "@/pages/MarketPage"
import PortfolioPage from "@/pages/PortfolioPage"
import FundDetailPage from "@/pages/FundDetailPage"

// bundle-dynamic-imports: 量化页面按需加载，减小初始 bundle
const BacktestPage = lazy(() => import("@/pages/BacktestPage"))
const DCAPage = lazy(() => import("@/pages/DCAPage"))
const ComparePage = lazy(() => import("@/pages/ComparePage"))

function App() {
    const fetchSessions = useSessionStore((s) => s.fetchSessions)

    useEffect(() => {
        fetchSessions()
    }, [fetchSessions])

    return (
        <BrowserRouter>
            <Suspense fallback={<div className="flex-1 flex items-center justify-center text-[var(--color-text-muted)]">加载中...</div>}>
                <Routes>
                    <Route element={<Layout />}>
                        <Route path="/" element={<Navigate to="/chat" replace />} />
                        <Route path="/chat" element={<ChatPage />} />
                        <Route path="/market" element={<MarketPage />} />
                        <Route path="/portfolio" element={<PortfolioPage />} />
                        <Route path="/fund/:code" element={<FundDetailPage />} />
                        <Route path="/backtest" element={<BacktestPage />} />
                        <Route path="/dca" element={<DCAPage />} />
                        <Route path="/compare" element={<ComparePage />} />
                    </Route>
                </Routes>
            </Suspense>
        </BrowserRouter>
    )
}

export default App
