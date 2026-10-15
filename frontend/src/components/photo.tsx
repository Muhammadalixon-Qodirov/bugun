import Image from "next/image";

import { mediaUrl } from "@/lib/media";
import { cn } from "@/lib/utils";

/**
 * A stored listing photo, filling its box. next/image serves it resized to the
 * shown width and as AVIF/WebP; cards pass `thumb` to start from the 480px copy.
 * `sizes` is the width it takes on screen, so small tiles load small files.
 */
export function Photo({
  k,
  alt,
  className,
  thumb,
  priority,
  sizes = thumb ? "(min-width: 640px) 18rem, 50vw" : "(min-width: 1024px) 50vw, 100vw",
  style,
}: {
  k: string;
  alt: string;
  className?: string;
  thumb?: boolean;
  priority?: boolean;
  sizes?: string;
  style?: React.CSSProperties;
}) {
  return (
    <span className="relative block size-full">
      <Image
        src={mediaUrl(k, thumb)}
        alt={alt}
        fill
        sizes={sizes}
        loading={priority ? "eager" : "lazy"}
        fetchPriority={priority ? "high" : "auto"}
        className={cn("object-cover", className)}
        style={style}
      />
    </span>
  );
}
