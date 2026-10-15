"use client";

import { useMutation } from "@tanstack/react-query";
import { LoaderCircle, Mic, Square } from "lucide-react";
import { useTranslations } from "next-intl";
import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { transcribePriceMutation } from "@/lib/api/gen/@tanstack/react-query.gen";
import type { TranscribePriceData } from "@/lib/api/gen/types.gen";
import { useApiErrors } from "@/lib/errors";
import { cn } from "@/lib/utils";

// Formats the API reads, in order of preference; browsers support different ones.
const mimeTypes = ["audio/webm;codecs=opus", "audio/webm", "audio/ogg;codecs=opus", "audio/mp4"];

const clock = (s: number) => `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;

/**
 * Records a short voice message in the browser and turns it into text: the
 * text is handed to `onText` for the seller to check before it is parsed.
 * Stops by itself at `maxSeconds`.
 */
export function VoiceInput({ maxSeconds, onText, disabled }: { maxSeconds: number; onText: (text: string) => void; disabled?: boolean }) {
  const t = useTranslations("price.voice");
  const errors = useApiErrors();
  const [seconds, setSeconds] = useState<number | null>(null);
  const recorder = useRef<MediaRecorder | null>(null);
  const timer = useRef<ReturnType<typeof setInterval> | null>(null);
  const started = useRef(0);

  const transcribe = useMutation({
    ...transcribePriceMutation(),
    onSuccess: (res) => {
      if (res.text.trim()) onText(res.text.trim());
      else toast.error(t("empty"));
    },
    onError: (err) => errors.show(err, "check"),
  });

  const cleanup = () => {
    if (timer.current) clearInterval(timer.current);
    timer.current = null;
    recorder.current?.stream.getTracks().forEach((tr) => tr.stop());
    recorder.current = null;
    setSeconds(null);
  };
  // Releases the microphone when the page is left mid-recording.
  useEffect(() => () => {
    if (timer.current) clearInterval(timer.current);
    recorder.current?.stream.getTracks().forEach((tr) => tr.stop());
  }, []);

  const stop = () => {
    if (recorder.current?.state === "recording") recorder.current.stop();
  };

  const start = async () => {
    if (typeof MediaRecorder === "undefined" || !navigator.mediaDevices?.getUserMedia) {
      toast.error(t("unsupported"));
      return;
    }
    let stream: MediaStream;
    try {
      stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    } catch {
      toast.error(t("denied"));
      return;
    }
    const mimeType = mimeTypes.find((m) => MediaRecorder.isTypeSupported(m));
    const rec = new MediaRecorder(stream, mimeType ? { mimeType } : undefined);
    const chunks: Blob[] = [];
    rec.ondataavailable = (e) => e.data.size > 0 && chunks.push(e.data);
    rec.onstop = () => {
      const duration = Math.max(1, Math.round((Date.now() - started.current) / 1000));
      const type = rec.mimeType || mimeType || "audio/webm";
      cleanup();
      if (!chunks.length) return;
      const ext = type.includes("ogg") ? "ogg" : type.includes("mp4") ? "m4a" : "webm";
      const file = new File(chunks, `voice.${ext}`, { type: type.split(";")[0] });
      // The API also takes the length the browser measured (the multipart field "duration").
      const body = { file, duration: String(Math.min(duration, maxSeconds)) } as TranscribePriceData["body"];
      transcribe.mutate({ body });
    };
    recorder.current = rec;
    started.current = Date.now();
    rec.start();
    setSeconds(0);
    timer.current = setInterval(() => {
      const s = Math.floor((Date.now() - started.current) / 1000);
      setSeconds(s);
      if (s >= maxSeconds) stop();
    }, 250);
  };

  const recording = seconds !== null;
  if (transcribe.isPending) {
    return (
      <Button type="button" variant="outline" size="pill" className="h-11" disabled>
        <LoaderCircle className="animate-spin" /> {t("transcribing")}
      </Button>
    );
  }
  return (
    <Button
      type="button"
      variant="outline"
      size="pill"
      className={cn("h-11", recording && "border-destructive/40 text-destructive hover:text-destructive")}
      disabled={disabled}
      onClick={recording ? stop : start}
      aria-label={recording ? t("stop") : t("start")}
      data-testid="voice-button"
    >
      {recording ? (
        <>
          <Square className="fill-current" />
          <span className="num" role="timer" aria-live="off">
            {clock(seconds)} / {clock(maxSeconds)}
          </span>
        </>
      ) : (
        <>
          <Mic /> {t("start")}
        </>
      )}
    </Button>
  );
}
