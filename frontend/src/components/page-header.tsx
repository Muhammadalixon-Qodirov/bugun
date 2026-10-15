import { Reveal } from "@/components/motion";

export function PageHeader({
  eyebrow,
  title,
  description,
  action,
}: {
  eyebrow?: React.ReactNode;
  title: React.ReactNode;
  description?: React.ReactNode;
  action?: React.ReactNode;
}) {
  return (
    <Reveal className="flex flex-wrap items-end justify-between gap-4">
      <div className="min-w-0">
        {eyebrow && <div className="eyebrow mb-3">{eyebrow}</div>}
        <h1 className="display text-3xl sm:text-[2.75rem]">{title}</h1>
        {description && <p className="mt-3 max-w-xl leading-relaxed text-muted-foreground sm:text-lg">{description}</p>}
      </div>
      {action}
    </Reveal>
  );
}
