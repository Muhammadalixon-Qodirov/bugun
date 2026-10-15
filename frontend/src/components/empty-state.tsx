import { cn } from "@/lib/utils";

/** The one "nothing here yet" block used across pages: optional icon, title, hint and action. */
export function EmptyState({
  icon,
  title,
  description,
  action,
  className,
}: {
  icon?: React.ReactNode;
  title: string;
  description?: React.ReactNode;
  action?: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex flex-col items-center rounded-3xl border border-dashed px-6 py-16 text-center", className)}>
      {icon && <span className="mb-5 grid size-12 place-items-center rounded-2xl bg-muted [&_svg]:size-5">{icon}</span>}
      <p className="text-lg font-semibold tracking-tight">{title}</p>
      {description && <p className="mt-1.5 max-w-sm text-sm leading-relaxed text-muted-foreground">{description}</p>}
      {action && <div className="mt-6">{action}</div>}
    </div>
  );
}
