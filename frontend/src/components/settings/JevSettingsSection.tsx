import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { Button, Card, CardContent, CardDescription, CardHeader, CardTitle, Label } from "@opskat/ui";
import { SecretInput } from "@/components/SecretInput";
import { notifySuccess } from "@/lib/notify";
import { GetJevAPIKey, SaveJevAPIKey } from "../../../wailsjs/go/system/System";

export function JevSettingsSection() {
  const { t } = useTranslation();
  const [key, setKey] = useState("");
  const [loaded, setLoaded] = useState(false);
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
    return () => {
      cancelled = true;
    };
  }, []);

  const save = async () => {
    setSaving(true);
    try {
      await SaveJevAPIKey(key);
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
        <p className="text-xs text-muted-foreground">{t("jev.hint")}</p>
        <Button data-testid="jev-save" size="sm" onClick={() => void save()} disabled={!loaded || saving}>
          {saving && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
          {t("action.save")}
        </Button>
      </CardContent>
    </Card>
  );
}
