import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Label } from "@opskat/ui";
import { SecretInput } from "@/components/SecretInput";
import { notifySuccess } from "@/lib/notify";
import {
  GetJevAPIKey,
  GetJevPrimaryConfidenceThreshold,
  SaveJevAPIKey,
  SaveJevPrimaryConfidenceThreshold,
} from "../../../wailsjs/go/system/System";

export function JevSettingsSection() {
  const { t } = useTranslation();
  const [key, setKey] = useState("");
  const [loaded, setLoaded] = useState(false);
  const [primaryConfidence, setPrimaryConfidence] = useState(50);
  const [thresholdLoaded, setThresholdLoaded] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    let cancelled = false;
    GetJevAPIKey()
      .then((value) => {
        if (!cancelled) {
          setKey(value ?? "");
          setLoaded(true);
        }
      })
      .catch((e: unknown) => {
        if (!cancelled) {
          toast.error(String(e));
          // A damaged stored key can still be replaced or removed.
          setLoaded(true);
        }
      });
    GetJevPrimaryConfidenceThreshold()
      .then((value) => {
        if (!cancelled) {
          const normalized = typeof value === "number" && Number.isFinite(value) ? value : 0.5;
          setPrimaryConfidence(Math.round(Math.min(1, Math.max(0.5, normalized)) * 100));
          setThresholdLoaded(true);
        }
      })
      .catch((e: unknown) => {
        if (!cancelled) {
          toast.error(String(e));
          setThresholdLoaded(true);
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const save = async () => {
    setSaving(true);
    try {
      await SaveJevAPIKey(key);
      await SaveJevPrimaryConfidenceThreshold(primaryConfidence / 100);
      notifySuccess(t("settings.saved"));
    } catch (e) {
      toast.error(String(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">{t("jev.title")}</CardTitle>
        <CardDescription>{t("jev.description")}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <Label htmlFor="jev-api-key">Jev API Key</Label>
        <SecretInput
          id="jev-api-key"
          data-testid="jev-api-key"
          value={key}
          onChange={(e) => setKey(e.target.value)}
          disabled={!loaded || saving}
          autoComplete="off"
        />
        {loaded && thresholdLoaded && key.trim() !== "" && (
          <div className="space-y-2" data-testid="jev-primary-confidence-setting">
            <div className="flex items-center justify-between gap-3">
              <Label htmlFor="jev-primary-confidence">{t("jev.primaryConfidence")}</Label>
              <span className="font-mono text-xs tabular-nums text-muted-foreground">{primaryConfidence}%</span>
            </div>
            <input
              id="jev-primary-confidence"
              data-testid="jev-primary-confidence"
              type="range"
              min={50}
              max={100}
              step={1}
              value={primaryConfidence}
              onChange={(e) => setPrimaryConfidence(Number(e.target.value))}
              disabled={saving}
              className="h-2 w-full cursor-pointer accent-primary"
            />
            <p className="text-xs text-muted-foreground">{t("jev.primaryConfidenceHint")}</p>
          </div>
        )}
        <p className="text-xs text-muted-foreground">{t("jev.hint")}</p>
        <Button
          data-testid="jev-save"
          size="sm"
          onClick={() => void save()}
          disabled={!loaded || !thresholdLoaded || saving}
        >
          {saving && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
          {t("action.save")}
        </Button>
      </CardContent>
    </Card>
  );
}
