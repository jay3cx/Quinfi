import * as React from "react"
import { cn } from "@/lib/utils"

export interface InputProps
    extends React.InputHTMLAttributes<HTMLInputElement> {
    variant?: "underline" | "boxed" | "ghost"
}

const Input = React.forwardRef<HTMLInputElement, InputProps>(
    ({ className, type, variant = "underline", ...props }, ref) => {
        return (
            <input
                type={type}
                className={cn(
                    "flex h-10 w-full bg-transparent px-3 py-2 text-sm ring-offset-background file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50 font-hand text-lg",
                    variant === "boxed" && "rounded-md border border-input focus-visible:border-[var(--color-primary)] focus-visible:ring-1 focus-visible:ring-[var(--color-primary)]/30",
                    variant === "underline" && "border-b-2 border-[var(--color-border)] focus:border-[var(--color-primary)] px-0 rounded-none transition-colors",
                    variant === "ghost" && "border-none shadow-none",
                    className
                )}
                ref={ref}
                {...props}
            />
        )
    }
)
Input.displayName = "Input"

export { Input }
