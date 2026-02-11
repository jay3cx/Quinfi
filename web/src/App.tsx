import { useEffect } from "react"
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom"
import { useSessionStore } from "@/stores/sessionStore"
import { Layout } from "@/components/Layout"
import ChatPage from "@/pages/ChatPage"
import MarketPage from "@/pages/MarketPage"
import PortfolioPage from "@/pages/PortfolioPage"
import FundDetailPage from "@/pages/FundDetailPage"

function App() {
    const fetchSessions = useSessionStore((s) => s.fetchSessions)

    useEffect(() => {
        fetchSessions()
    }, [fetchSessions])

    return (
        <BrowserRouter>
            <Routes>
                <Route element={<Layout />}>
                    <Route path="/" element={<Navigate to="/chat" replace />} />
                    <Route path="/chat" element={<ChatPage />} />
                    <Route path="/market" element={<MarketPage />} />
                    <Route path="/portfolio" element={<PortfolioPage />} />
                    <Route path="/fund/:code" element={<FundDetailPage />} />
                </Route>
            </Routes>
        </BrowserRouter>
    )
}

export default App
