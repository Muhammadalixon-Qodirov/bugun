"use client";

import { useMutation } from "@tanstack/react-query";
import { CalendarRange, Circle, Clock, ExternalLink, MapPin, Pencil, Plus, Search, Trash2 } from "lucide-react";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState } from "react";
import { toast } from "sonner";

import { EventForm } from "@/components/admin/event-form";
import { eventKindIcons } from "@/components/admin/event-kind";
import { useCatalogNames } from "@/components/catalog-names";
import { DemoBadge } from "@/components/demo-badge";
import { EmptyState } from "@/components/empty-state";
import { Button } from "@/components/ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { adminDeleteEventMutation } from "@/lib/api/gen/@tanstack/react-query.gen";
import type { MarketEvent } from "@/lib/api/gen/types.gen";
import { useApiErrors } from "@/lib/errors";
import { useFormat } from "@/lib/i18n/use-format";
import type { AdminCatalog } from "@/lib/server/admin";

function norm(s: string) {
  return s.toLowerCase().replace(/[‘’`ʻʼ']/g, "");
}

/** The market events journal: search, a table (cards on phones), and create, edit and delete dialogs. */
export function AdminEvents({ events, catalog }: { events: MarketEvent[]; catalog: AdminCatalog }) {
  const router = useRouter();
  const t = useTranslations("admin.events");
  const [query, setQuery] = useState("");
  // "new" opens an empty form; an event opens it filled in. The target outlives
  // the open flag so a closing dialog keeps its content while it animates out.
  const [editing, setEditing] = useState<{ open: boolean; event?: MarketEvent; n: number }>({ open: false, n: 0 });
  const [deleting, setDeleting] = useState<{ open: boolean; event?: MarketEvent }>({ open: false });
  const edit = (event?: MarketEvent) => setEditing((s) => ({ open: true, event, n: s.n + 1 }));
  const remove = (event: MarketEvent) => setDeleting({ open: true, event });
  const visible = query ? events.filter((e) => norm(`${e.title} ${e.note ?? ""}`).includes(norm(query))) : events;

  const done = () => {
    setEditing((s) => ({ ...s, open: false }));
    router.refresh();
  };

  return (
    <div>
      <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div className="max-w-xl text-sm text-muted-foreground">
          <p>{t("intro")}</p>
          <p className="mt-1.5 flex items-center gap-1.5 text-xs">
            <Clock className="size-3.5 shrink-0" /> {t("cacheNote")}
          </p>
        </div>
        <Button variant="solid" size="pill" className="self-start sm:self-auto" onClick={() => edit()}>
          <Plus /> {t("add")}
        </Button>
      </div>

      {events.length > 0 && (
        <label className="mt-5 flex h-10 items-center gap-2 rounded-full bg-muted px-3.5 sm:w-72">
          <Search className="size-4 shrink-0 text-muted-foreground" />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t("searchPlaceholder")}
            aria-label={t("searchLabel")}
            className="w-full min-w-0 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
          />
        </label>
      )}

      <div className="mt-4">
        {events.length === 0 ? (
          <EmptyState icon={<CalendarRange strokeWidth={1.5} />} title={t("emptyTitle")} description={t("emptyDescription")} />
        ) : visible.length === 0 ? (
          <p className="rounded-3xl border px-5 py-14 text-center text-sm text-muted-foreground">{t("emptySearch", { query })}</p>
        ) : (
          <>
            <EventTable events={visible} catalog={catalog} onEdit={edit} onDelete={remove} />
            <ul className="space-y-3 md:hidden">
              {visible.map((e) => (
                <li key={e.id}>
                  <EventCard e={e} catalog={catalog} onEdit={edit} onDelete={remove} />
                </li>
              ))}
            </ul>
          </>
        )}
      </div>

      <Dialog open={editing.open} onOpenChange={(open) => setEditing((s) => ({ ...s, open }))}>
        <DialogContent className="sm:w-[min(36rem,calc(100vw-2rem))]">
          <EventForm key={editing.n} event={editing.event} catalog={catalog} onDone={done} />
        </DialogContent>
      </Dialog>
      <DeleteDialog {...deleting} onClose={() => setDeleting((s) => ({ ...s, open: false }))} />
    </div>
  );
}

type RowProps = { e: MarketEvent; catalog: AdminCatalog; onEdit: (e: MarketEvent) => void; onDelete: (e: MarketEvent) => void };

/** Dates, place and scope of an event in the visitor's language. */
function useEventText(catalog: AdminCatalog) {
  const t = useTranslations("admin.events");
  const tcodes = useTranslations("codes");
  const tc = useTranslations("common");
  const fmt = useFormat();
  const names = useCatalogNames();
  return {
    kind: (e: MarketEvent) => {
      const key = `eventKind.${e.kind}` as Parameters<typeof tcodes>[0];
      return tcodes.has(key) ? tcodes(key) : e.kind;
    },
    // Day strings are Tashkent days already; the year is added once for a range within one year.
    dates: (e: MarketEvent) => {
      if (!e.ends_on || e.ends_on === e.starts_on) return fmt.date(e.starts_on);
      const year = e.starts_on.slice(0, 4);
      return year === e.ends_on.slice(0, 4) ? `${fmt.range(e.starts_on, e.ends_on)}, ${year}` : `${fmt.date(e.starts_on)} – ${fmt.date(e.ends_on)}`;
    },
    region: (e: MarketEvent) => {
      const name = e.province ?? catalog.provinces.find((p) => p.id === e.province_id)?.name;
      return name ? names.region(name) : tc("wholeCountry");
    },
    target: (e: MarketEvent) => {
      if (e.product_name) return names.product(undefined, e.product_name);
      const category = catalog.categories.find((c) => c.slug === e.category);
      return category ? names.category(category.slug, category.name) : t("allProducts");
    },
  };
}

function Actions({ e, onEdit, onDelete }: Omit<RowProps, "catalog">) {
  const t = useTranslations("admin.events");
  return (
    <div className="flex justify-end gap-1">
      <Button variant="ghost" size="icon-lg" className="size-10 rounded-full" aria-label={t("editLabel", { title: e.title })} onClick={() => onEdit(e)}>
        <Pencil />
      </Button>
      <Button
        variant="ghost"
        size="icon-lg"
        className="size-10 rounded-full text-muted-foreground hover:text-destructive"
        aria-label={t("deleteLabel", { title: e.title })}
        onClick={() => onDelete(e)}
      >
        <Trash2 />
      </Button>
    </div>
  );
}

function SourceLink({ e }: { e: MarketEvent }) {
  const t = useTranslations("admin.events");
  if (!e.source_url) return null;
  return (
    <a
      href={e.source_url}
      target="_blank"
      rel="noopener noreferrer"
      className="inline-flex items-center gap-1 text-sm text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
    >
      {t("sourceLink")} <ExternalLink className="size-3.5" />
    </a>
  );
}

function KindChip({ e, text }: { e: MarketEvent; text: ReturnType<typeof useEventText> }) {
  const Icon = eventKindIcons[e.kind] ?? Circle;
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full bg-muted px-2.5 py-1 text-xs font-medium">
      <Icon className="size-3.5 text-muted-foreground" aria-hidden />
      {text.kind(e)}
    </span>
  );
}

function EventTable({ events, catalog, onEdit, onDelete }: { events: MarketEvent[] } & Omit<RowProps, "e">) {
  const t = useTranslations("admin.events.columns");
  const text = useEventText(catalog);
  return (
    <div className="hidden overflow-hidden rounded-3xl border bg-card md:block">
      <Table>
        <TableHeader>
          <TableRow className="text-xs hover:bg-transparent [&>th]:text-muted-foreground">
            <TableHead className="pl-5">{t("kind")}</TableHead>
            <TableHead>{t("title")}</TableHead>
            <TableHead>{t("dates")}</TableHead>
            <TableHead>{t("region")}</TableHead>
            <TableHead>{t("target")}</TableHead>
            <TableHead>{t("source")}</TableHead>
            <TableHead className="pr-5">
              <span className="sr-only">{t("actions")}</span>
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {events.map((e) => (
            <TableRow key={e.id}>
              <TableCell className="py-3 pl-5">
                <KindChip e={e} text={text} />
              </TableCell>
              <TableCell className="max-w-80 min-w-56 whitespace-normal">
                <p className="font-medium">
                  {e.title}
                  {e.demo && <DemoBadge className="ml-2 py-0.5 align-middle" />}
                </p>
                {e.note && <p className="mt-0.5 line-clamp-1 text-xs text-muted-foreground">{e.note}</p>}
              </TableCell>
              <TableCell className="text-muted-foreground">{text.dates(e)}</TableCell>
              <TableCell className="text-muted-foreground">{text.region(e)}</TableCell>
              <TableCell className="text-muted-foreground">{text.target(e)}</TableCell>
              <TableCell>
                <SourceLink e={e} />
              </TableCell>
              <TableCell className="py-1 pr-4">
                <Actions e={e} onEdit={onEdit} onDelete={onDelete} />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

function EventCard({ e, catalog, onEdit, onDelete }: RowProps) {
  const text = useEventText(catalog);
  return (
    <article className="rounded-3xl border bg-card p-4">
      <div className="flex items-start justify-between gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <KindChip e={e} text={text} />
          {e.demo && <DemoBadge className="py-0.5" />}
        </div>
        <div className="-mt-1.5 -mr-1.5">
          <Actions e={e} onEdit={onEdit} onDelete={onDelete} />
        </div>
      </div>
      <p className="mt-2 font-medium">{e.title}</p>
      {e.note && <p className="mt-0.5 line-clamp-2 text-sm text-muted-foreground">{e.note}</p>}
      <div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-muted-foreground">
        <span className="flex items-center gap-1">
          <CalendarRange className="size-3.5" /> {text.dates(e)}
        </span>
        <span className="flex items-center gap-1">
          <MapPin className="size-3.5" /> {text.region(e)}
        </span>
        <span>{text.target(e)}</span>
        <SourceLink e={e} />
      </div>
    </article>
  );
}

function DeleteDialog({ open, event, onClose }: { open: boolean; event?: MarketEvent; onClose: () => void }) {
  const router = useRouter();
  const t = useTranslations("admin.events");
  const tc = useTranslations("common");
  const errors = useApiErrors();
  const remove = useMutation({
    ...adminDeleteEventMutation(),
    onSuccess: () => {
      toast.success(t("deleted"));
      onClose();
      router.refresh();
    },
    onError: (err) => errors.show(err, "delete"),
  });
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        {event && (
          <>
            <DialogTitle>{t("deleteTitle")}</DialogTitle>
            <DialogDescription>{t("deleteDescription", { title: event.title })}</DialogDescription>
            <div className="mt-6 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
              <DialogClose render={<Button type="button" variant="ghost" size="pill" />}>{tc("cancel")}</DialogClose>
              <Button variant="destructive" size="pill" disabled={remove.isPending} onClick={() => remove.mutate({ path: { id: event.id } })}>
                <Trash2 /> {tc("delete")}
              </Button>
            </div>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
