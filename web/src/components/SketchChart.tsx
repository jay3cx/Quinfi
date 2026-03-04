import { useEffect, useRef, useState, useCallback } from "react"
import rough from "roughjs"

interface DataPoint {
    label: string
    value: number
}

interface SketchChartProps {
    data: DataPoint[]
    color?: string
    height?: number
    className?: string
}

interface HoverInfo {
    dataX: number
    dataY: number
    label: string
    value: number
    index: number
}

export function SketchChart({
    data,
    color = "var(--color-primary)",
    height = 200,
    className
}: SketchChartProps) {
    const canvasRef = useRef<HTMLCanvasElement>(null)
    const containerRef = useRef<HTMLDivElement>(null)
    const [hover, setHover] = useState<HoverInfo | null>(null)

    const layoutRef = useRef<{
        padding: { top: number; right: number; bottom: number; left: number }
        chartW: number
        chartH: number
        yMin: number
        yRange: number
        getX: (i: number) => number
        getY: (v: number) => number
    } | null>(null)

    useEffect(() => {
        if (!canvasRef.current || !containerRef.current || data.length < 2) return

        const canvas = canvasRef.current
        const container = containerRef.current
        const rc = rough.canvas(canvas)

        const draw = () => {
            const width = container.offsetWidth
            const dpr = window.devicePixelRatio || 1
            canvas.width = width * dpr
            canvas.height = height * dpr

            const ctx = canvas.getContext("2d")
            if (!ctx) return

            ctx.scale(dpr, dpr)
            canvas.style.width = `${width}px`
            canvas.style.height = `${height}px`
            ctx.clearRect(0, 0, width, height)

            const padding = { top: 20, right: 20, bottom: 30, left: 40 }
            const chartW = width - padding.left - padding.right
            const chartH = height - padding.top - padding.bottom

            const yMax = Math.max(...data.map(d => d.value))
            const yMin = Math.min(...data.map(d => d.value))
            const yRange = yMax - yMin || 1

            const getX = (index: number) => padding.left + (index / (data.length - 1)) * chartW
            const getY = (val: number) => padding.top + chartH - ((val - yMin) / yRange) * chartH

            layoutRef.current = { padding, chartW, chartH, yMin, yRange, getX, getY }

            // 手绘风坐标轴
            rc.line(padding.left, padding.top - 5, padding.left, height - padding.bottom, {
                stroke: "#d1d5db", roughness: 0.8, bowing: 1
            })
            rc.line(padding.left, height - padding.bottom, width - padding.right, height - padding.bottom, {
                stroke: "#d1d5db", roughness: 0.8, bowing: 1
            })

            // 解析颜色
            let strokeColor = color
            if (color.startsWith("var(")) {
                strokeColor = getComputedStyle(document.documentElement).getPropertyValue(
                    color.substring(4, color.length - 1)
                ).trim() || "#166534"
            }

            // 平滑曲线（Canvas 原生，不用 RoughJS）
            const points = data.map((d, i) => ({ x: getX(i), y: getY(d.value) }))

            // 渐变填充
            const gradient = ctx.createLinearGradient(0, padding.top, 0, height - padding.bottom)
            gradient.addColorStop(0, strokeColor + "18")
            gradient.addColorStop(1, strokeColor + "02")

            ctx.beginPath()
            ctx.moveTo(points[0].x, points[0].y)
            for (let i = 1; i < points.length; i++) {
                const prev = points[i - 1]
                const curr = points[i]
                const cpx = (prev.x + curr.x) / 2
                ctx.quadraticCurveTo(prev.x + (cpx - prev.x) * 0.5, prev.y, cpx, (prev.y + curr.y) / 2)
                ctx.quadraticCurveTo(curr.x - (curr.x - cpx) * 0.5, curr.y, curr.x, curr.y)
            }

            // 填充区域
            ctx.lineTo(points[points.length - 1].x, height - padding.bottom)
            ctx.lineTo(points[0].x, height - padding.bottom)
            ctx.closePath()
            ctx.fillStyle = gradient
            ctx.fill()

            // 线条
            ctx.beginPath()
            ctx.moveTo(points[0].x, points[0].y)
            for (let i = 1; i < points.length; i++) {
                const prev = points[i - 1]
                const curr = points[i]
                const cpx = (prev.x + curr.x) / 2
                ctx.quadraticCurveTo(prev.x + (cpx - prev.x) * 0.5, prev.y, cpx, (prev.y + curr.y) / 2)
                ctx.quadraticCurveTo(curr.x - (curr.x - cpx) * 0.5, curr.y, curr.x, curr.y)
            }
            ctx.strokeStyle = strokeColor
            ctx.lineWidth = 1.5
            ctx.lineJoin = "round"
            ctx.lineCap = "round"
            ctx.stroke()

            // 标签
            ctx.font = '11px "Inter", sans-serif'
            ctx.fillStyle = "#9B9590"
            ctx.textAlign = "center"

            // X 轴标签 — 均匀分布几个
            const labelCount = Math.min(5, data.length)
            for (let i = 0; i < labelCount; i++) {
                const idx = Math.round((i / (labelCount - 1)) * (data.length - 1))
                ctx.fillText(data[idx].label, getX(idx), height - 8)
            }

            // Y 轴标签
            ctx.textAlign = "right"
            ctx.fillText(yMax.toFixed(2), padding.left - 5, padding.top + 4)
            ctx.fillText(yMin.toFixed(2), padding.left - 5, height - padding.bottom - 2)
        }

        draw()
        const observer = new ResizeObserver(draw)
        observer.observe(container)
        return () => observer.disconnect()

    }, [data, color, height])

    const handleMouseMove = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
        const layout = layoutRef.current
        if (!layout || data.length < 2 || !containerRef.current) return

        const rect = containerRef.current.getBoundingClientRect()
        const mouseX = e.clientX - rect.left

        let nearestIdx = 0
        let minDist = Infinity
        for (let i = 0; i < data.length; i++) {
            const dist = Math.abs(mouseX - layout.getX(i))
            if (dist < minDist) {
                minDist = dist
                nearestIdx = i
            }
        }

        setHover({
            dataX: layout.getX(nearestIdx),
            dataY: layout.getY(data[nearestIdx].value),
            label: data[nearestIdx].label,
            value: data[nearestIdx].value,
            index: nearestIdx,
        })
    }, [data])

    const handleMouseLeave = useCallback(() => {
        setHover(null)
    }, [])

    return (
        <div ref={containerRef} className={`relative ${className || ""}`}>
            <canvas ref={canvasRef} />

            <div
                className="absolute inset-0 cursor-crosshair"
                onMouseMove={handleMouseMove}
                onMouseLeave={handleMouseLeave}
            />

            {hover && (
                <>
                    {/* 垂直虚线 */}
                    <div
                        className="absolute top-5 pointer-events-none"
                        style={{
                            left: hover.dataX,
                            height: height - 50,
                            borderLeft: "1px dashed #E8E5DE",
                        }}
                    />

                    {/* 数据点 */}
                    <div
                        className="absolute w-2.5 h-2.5 rounded-full pointer-events-none"
                        style={{
                            left: hover.dataX - 5,
                            top: hover.dataY - 5,
                            backgroundColor: "var(--color-primary)",
                            boxShadow: "0 0 0 3px white, 0 0 0 4px var(--color-primary)",
                        }}
                    />

                    {/* 浮窗 */}
                    <div
                        className="absolute pointer-events-none z-10"
                        style={{
                            left: hover.dataX + (hover.index > data.length * 0.7 ? -120 : 12),
                            top: Math.max(0, hover.dataY - 36),
                        }}
                    >
                        <div className="bg-white rounded-lg shadow-md border border-[var(--color-border)] px-3 py-1.5">
                            <div className="text-[10px] text-[var(--color-text-muted)]">{hover.label}</div>
                            <div className="text-sm font-semibold text-[var(--color-text)] tabular-nums">
                                {hover.value.toFixed(4)}
                            </div>
                        </div>
                    </div>
                </>
            )}
        </div>
    )
}
