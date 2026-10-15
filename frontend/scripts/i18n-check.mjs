// Checks that the translations stay in step with Uzbek, the source language:
// messages/en and messages/ru have exactly the namespaces and keys of
// messages/uz, every message parses and uses the same {arguments} and <tags>,
// the generated index files are current, and the catalog name files cover the
// same slugs.
import { readdirSync, readFileSync } from "node:fs";

import { IntlMessageFormat } from "intl-messageformat";

import { indexSource } from "./i18n-index.mjs";

const dir = new URL("../src/messages/", import.meta.url);
const read = (name) => JSON.parse(readFileSync(new URL(name, dir), "utf8"));

/** All namespace files of a locale as one object, like src/messages/<locale>/index.ts. */
function readLocale(locale) {
  const files = readdirSync(new URL(`${locale}/`, dir)).filter((f) => f.endsWith(".json"));
  return Object.fromEntries(files.map((f) => [f.slice(0, -5), read(`${locale}/${f}`)]));
}

function flatten(node, prefix = "", out = new Map()) {
  for (const [k, v] of Object.entries(node)) {
    const key = prefix ? `${prefix}.${k}` : k;
    if (typeof v === "string") out.set(key, v);
    else flatten(v, key, out);
  }
  return out;
}

// Arguments a message refers to: shown ({name}) and plural/select selectors
// ({count, plural, …}), plus <tags>. Uzbek has no plural forms, so "{count} kun"
// pairs with "{count, plural, one {# day} …}" and "{formatted} ta" with
// "{count, plural, one {{formatted} listing} …}".
function args(msg) {
  const shown = new Set([...msg.matchAll(/\{\s*([A-Za-z_]\w*)\s*\}/g)].map((m) => m[1]));
  const selectors = new Set([...msg.matchAll(/\{\s*([A-Za-z_]\w*)\s*,\s*(?:plural|select|selectordinal)\s*,/g)].map((m) => m[1]));
  const tags = new Set([...msg.matchAll(/<(\w+)>/g)].map((m) => m[1]));
  return { shown, all: new Set([...shown, ...selectors]), tags };
}

function mismatch(uz, other) {
  const a = args(uz);
  const b = args(other);
  const missing = [...a.all].filter((n) => !b.all.has(n));
  const extra = [...b.shown].filter((n) => !a.all.has(n));
  const tags = [...a.tags].sort().join() !== [...b.tags].sort().join();
  if (missing.length) return `does not use {${missing.join("}, {")}}`;
  if (extra.length) return `uses {${extra.join("}, {")}} that uz does not have`;
  if (tags) return "has different <tags> from uz";
  return null;
}

// Parses and renders a message with sample values. A stray apostrophe before
// "{" quotes the brace instead of failing, so leftover braces count as broken.
function broken(msg, locale) {
  try {
    const values = Object.fromEntries([...args(msg).all].map((n) => [n, 2]));
    for (const tag of args(msg).tags) values[tag] = (chunks) => chunks.join("");
    const out = [new IntlMessageFormat(msg, locale).format(values)].flat().join("");
    return /[{}]/.test(out) ? `renders as "${out}"` : null;
  } catch (e) {
    return `does not parse: ${e.message}`;
  }
}

const problems = [];
const source = flatten(readLocale("uz"));
for (const [key, msg] of source) {
  const why = broken(msg, "uz");
  if (why) problems.push(`uz: ${key} ${why}`);
}
for (const locale of ["en", "ru"]) {
  const target = flatten(readLocale(locale));
  for (const [key, msg] of source) {
    if (!target.has(key)) problems.push(`${locale}: missing ${key}`);
    else if (!target.get(key).trim()) problems.push(`${locale}: ${key} is empty`);
    else {
      const why = broken(target.get(key), locale) ?? mismatch(msg, target.get(key));
      if (why) problems.push(`${locale}: ${key} ${why}`);
    }
  }
  for (const key of target.keys()) if (!source.has(key)) problems.push(`${locale}: ${key} is not in messages/uz`);
}

for (const locale of ["uz", "en", "ru"]) {
  let current = "";
  try {
    current = readFileSync(new URL(`${locale}/index.ts`, dir), "utf8");
  } catch {}
  if (current !== indexSource(locale)) problems.push(`${locale}/index.ts is out of date: run npm run i18n:index`);
}

const en = read("catalog.en.json");
const ru = read("catalog.ru.json");
for (const group of new Set([...Object.keys(en), ...Object.keys(ru)])) {
  const a = Object.keys(en[group] ?? {});
  const b = Object.keys(ru[group] ?? {});
  for (const slug of a) if (!(ru[group] ?? {})[slug]) problems.push(`catalog.ru: missing ${group}.${slug}`);
  for (const slug of b) if (!(en[group] ?? {})[slug]) problems.push(`catalog.en: missing ${group}.${slug}`);
}

if (problems.length) {
  console.error(problems.join("\n"));
  console.error(`\n${problems.length} translation problem(s).`);
  process.exit(1);
}
console.log(`i18n ok: ${source.size} messages in uz, en, ru; catalog names in step.`);
