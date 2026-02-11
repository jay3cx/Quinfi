import * as React from "react"
import rough from "roughjs"
import { cn } from "@/lib/utils"

const CardContext = React.createContext<{ variant?: "default" | "sketchy" }>({ variant: "default" })

interface CardProps extends React.HTMLAttributes<HTMLDivElement> {
    variant?: "default" | "sketchy"
}

const Card = React.forwardRef<HTMLDivElement, CardProps>(
    ({ className, variant = "default", children, ...props }, ref) => {
        const containerRef = React.useRef<HTMLDivElement>(null)
        const canvasRef = React.useRef<HTMLCanvasElement>(null)

        // Merge internal ref with forwarded ref
        // Standard react practice is useImperativeHandle but simple assignment works for DOM refs usually
        // Let's stick to simple composition

        React.useEffect(() => {
            if (variant !== "sketchy" || !canvasRef.current || !containerRef.current) return

            const container = containerRef.current
            const canvas = canvasRef.current
            const rc = rough.canvas(canvas)

            const draw = () => {
                const { offsetWidth: w, offsetHeight: h } = container
                canvas.width = w
                canvas.height = h

                // Clear
                const ctx = canvas.getContext("2d")
                ctx?.clearRect(0, 0, w, h)

                // Draw
                // We start slightly inside to account for the stroke
                rc.rectangle(2, 2, w - 4, h - 4, {
                    roughness: 1.2,
                    bowing: 1.5,
                    stroke: "#cbd5e1", // slate-300
                    strokeWidth: 1.5,
                    disableMultiStroke: false,
                })
            }

            draw()

            const observer = new ResizeObserver(draw)
            observer.observe(container)

            return () => observer.disconnect()
        }, [variant])

        return (
            <CardContext.Provider value={{ variant }}>
                <div
                    ref={containerRef}
                    className={cn(
                        "relative rounded-lg bg-white text-slate-950 shadow-sm",
                        variant === "default" && "border border-slate-200",
                        variant === "sketchy" && "bg-white/80 backdrop-blur-sm", // Slight transparency for cool effect
                        className
                    )}
                    {...props}
                >
                    <div ref={ref /* Forward ref mainly covers content area */}>
                        {children}
                    </div>

                    {variant === "sketchy" && (
                        <canvas
                            ref={canvasRef}
                            className="pointer-events-none absolute inset-0 z-0"
                            style={{ width: '100%', height: '100%' }}
                        />
                    )}
                </div>
            </CardContext.Provider>
        )
    }
)
Card.displayName = "Card"

const CardHeader = React.forwardRef<
    HTMLDivElement,
    React.HTMLAttributes<HTMLDivElement>
>(({ className, ...props }, ref) => (
    <div
        ref={ref}
        className={cn("flex flex-col space-y-1.5 p-6 relative z-10", className)}
        {...props}
    />
))
CardHeader.displayName = "CardHeader"

const CardTitle = React.forwardRef<
    HTMLParagraphElement,
    React.HTMLAttributes<HTMLHeadingElement>
>(({ className, ...props }, ref) => (
    <h3
        ref={ref}
        className={cn(
            "text-2xl font-semibold leading-none tracking-tight font-serif text-[var(--color-primary)]",
            className
        )}
        {...props}
    />
))
CardTitle.displayName = "CardTitle"

const CardDescription = React.forwardRef<
    HTMLParagraphElement,
    React.HTMLAttributes<HTMLParagraphElement>
>(({ className, ...props }, ref) => (
    <p
        ref={ref}
        className={cn("text-sm text-slate-500", className)}
        {...props}
    />
))
CardDescription.displayName = "CardDescription"

const CardContent = React.forwardRef<
    HTMLDivElement,
    React.HTMLAttributes<HTMLDivElement>
>(({ className, ...props }, ref) => (
    <div ref={ref} className={cn("p-6 pt-0 relative z-10", className)} {...props} />
))
CardContent.displayName = "CardContent"

const CardFooter = React.forwardRef<
    HTMLDivElement,
    React.HTMLAttributes<HTMLDivElement>
>(({ className, ...props }, ref) => (
    <div
        ref={ref}
        className={cn("flex items-center p-6 pt-0 relative z-10", className)}
        {...props}
    />
))
CardFooter.displayName = "CardFooter"

export { Card, CardHeader, CardFooter, CardTitle, CardDescription, CardContent }
