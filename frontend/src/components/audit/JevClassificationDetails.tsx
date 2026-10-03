import { useTranslation } from "react-i18next";

const primaryTypes = ["SAFE_READ", "SENSITIVE_READ", "SAFE_CHANGE", "DANGEROUS_CHANGE", "UNKNOWN"];
interface Score {
  type: string;
  probability: number;
}

function object(value: unknown): Record<string, unknown> {
  if (value === null || typeof value !== "object" || Array.isArray(value))
    throw new Error("invalid classification object");
  return value as Record<string, unknown>;
}

function probability(value: unknown): number {
  if (typeof value !== "number" || !Number.isFinite(value) || value < 0 || value > 1)
    throw new Error("invalid classification probability");
  return value;
}

function decode(source: string) {
  const data = object(JSON.parse(source));
  const primary: Score[] = [];
  const secondary: Score[] = [];
  if (data.primary != null) {
    const scores = object(object(data.primary).probabilities);
    for (const type of primaryTypes) primary.push({ type, probability: probability(scores[type]) });
  }
  if (data.secondary != null) {
    for (const [type, answer] of Object.entries(object(data.secondary)))
      secondary.push({ type, probability: probability(object(answer).noul) });
  }
  return {
    primary,
    secondary,
    status: typeof data.status === "string" ? data.status : "",
    reason: typeof data.reason === "string" ? data.reason : "",
  };
}

function ProbabilityBar({ score, secondary }: { score: Score; secondary: boolean }) {
  const { t } = useTranslation();
  const label = t(`${secondary ? "commandSubtype" : "commandType"}.${score.type}`, { defaultValue: score.type });
  const percent = Math.round(score.probability * 1000) / 10;
  return (
    <div className="space-y-1">
      <div className="flex items-start justify-between gap-3 text-xs">
        <span>{label}</span>
        <span className="shrink-0 font-mono tabular-nums text-muted-foreground">{percent}%</span>
      </div>
      <div
        role="progressbar"
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={percent}
        className="h-2 overflow-hidden rounded-full bg-muted"
      >
        <div className="h-full rounded-full bg-primary" style={{ width: `${percent}%` }} />
      </div>
    </div>
  );
}

export function JevClassificationDetails({ source }: { source: string }) {
  const { t } = useTranslation();
  let data: ReturnType<typeof decode>;
  try {
    data = decode(source);
  } catch {
    return (
      <p role="alert" className="text-xs text-destructive">
        {t("audit.classificationInvalid")}
      </p>
    );
  }
  return (
    <div
      data-testid="jev-classification-details"
      className="space-y-4 rounded-md border border-border bg-background p-3"
    >
      {data.primary.length > 0 ? (
        <section className="space-y-3">
          <div className="text-xs font-medium">{t("audit.primaryClassification")}</div>
          {data.primary.map((score) => (
            <ProbabilityBar key={score.type} score={score} secondary={false} />
          ))}
        </section>
      ) : (
        <p className="text-xs text-muted-foreground">{t("audit.classificationNoProbabilities")}</p>
      )}
      {data.secondary.length > 0 && (
        <section className="space-y-3 border-t border-border pt-3">
          <div className="text-xs font-medium">{t("audit.secondaryClassification")}</div>
          {data.secondary.map((score) => (
            <ProbabilityBar key={score.type} score={score} secondary />
          ))}
        </section>
      )}
      {data.status && data.status !== "OK" && (
        <p className="text-xs text-muted-foreground">
          {t(`audit.classificationStatus.${data.status}`, { defaultValue: data.status })}
          {data.reason && ` · ${data.reason}`}
        </p>
      )}
    </div>
  );
}
