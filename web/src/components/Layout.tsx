import { Outlet } from "react-router-dom"
import { Sidebar } from "@/components/Sidebar"

export function Layout() {
    return (
        <div className="flex min-h-screen bg-[var(--color-bg)] font-sans">
            <Sidebar />
            <main className="flex-1 flex flex-col h-screen relative">
                <Outlet />
            </main>
        </div>
    )
}
