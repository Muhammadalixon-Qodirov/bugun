import "server-only";

import { unstable_cache } from "next/cache";
import { cache } from "react";

import { aiFeatures } from "@/lib/api/gen/sdk.gen";
import type { AiFeatures } from "@/lib/api/gen/types.gen";
import { LIMITS_MAX_AGE_SECONDS } from "@/lib/limits";

/** Everything off: an API without GET /v1/ai/features, or one that could not be asked. */
export const AI_OFF: AiFeatures = {
  voice: false,
  voice_max_seconds: 0,
  photo: false,
  analyst_model: false,
  embeddings: false,
  moderation_model: false,
  advice_model: false,
};

class FeaturesUnavailable extends Error {}

// The switches change only with an API restart, like the limits. A failed
// request throws inside, so it is not cached and the next page asks again.
const cachedFeatures = unstable_cache(
  async () => {
    const { data, response } = await aiFeatures({ signal: AbortSignal.timeout(3000) });
    if (data) return data;
    // An older API without the endpoint has none of these features: cache that.
    if (response?.status === 404) return AI_OFF;
    throw new FeaturesUnavailable(`Could not load the AI features: ${response?.status ?? "no response"}`);
  },
  ["ai-features-v1"],
  { revalidate: LIMITS_MAX_AGE_SECONDS },
);

/** Which optional features (voice, photo, …) the API has turned on; all off when unknown. */
export const getAiFeatures = cache(async (): Promise<AiFeatures> => cachedFeatures().catch(() => AI_OFF));
