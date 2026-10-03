import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { BookOpen, Loader2 } from "lucide-react";
import Markdown from "react-markdown";
import { Button } from "@opskat/ui";
import { markdownComponents, markdownUrlTransform } from "@/components/MarkdownLink";
import { ExplainCommand, CancelCommandExplanation } from "../../../wailsjs/go/ai/AI";
import { EventsOn } from "../../../wailsjs/runtime/runtime";
import { toast } from "sonner";

interface ExplainableCommand {
  type: string;
  command: string;
  asset_name?: string;
  detail?: string;
}

interface ExplanationRequest {
  id: string;
  unsubscribe: () => void;
  started: boolean;
  finished: boolean;
  cancelled: boolean;
  lastSequence: number;
}

async function stopRequest(request: ExplanationRequest) {
  request.unsubscribe();
  if (request.started && !request.finished && !request.cancelled) {
    request.cancelled = true;
    await CancelCommandExplanation(request.id);
  }
}

export function CommandExplanation({ item }: { item: ExplainableCommand }) {
  const { t } = useTranslation();
  const [answer, setAnswer] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const generation = useRef(0);
  const activeRequest = useRef<ExplanationRequest | null>(null);
  useEffect(
    () => () => {
      generation.current++;
      if (activeRequest.current) void stopRequest(activeRequest.current).catch((e) => toast.error(String(e)));
    },
    []
  );

  const explain = async () => {
    const current = ++generation.current;
    const request: ExplanationRequest = {
      id: crypto.randomUUID(),
      unsubscribe: () => {},
      started: false,
      finished: false,
      cancelled: false,
      lastSequence: 0,
    };
    setLoading(true);
    setError("");
    setAnswer("");
    request.unsubscribe = EventsOn(
      `ai:command-explanation:${request.id}`,
      (event: { sequence: number; content: string; error?: string; done?: boolean }) => {
        if (current !== generation.current || request.finished || event.sequence <= request.lastSequence) return;
        request.lastSequence = event.sequence;
        setAnswer(event.content);
        if (event.done) {
          request.finished = true;
          request.unsubscribe();
          activeRequest.current = null;
          if (event.error) setError(event.error);
          setLoading(false);
        }
      }
    );
    activeRequest.current = request;
    try {
      await ExplainCommand(request.id, {
        type: item.type,
        command: item.command,
        asset_name: item.asset_name ?? "",
        detail: item.detail ?? "",
      });
      request.started = true;
      if (current !== generation.current) await stopRequest(request);
    } catch (e) {
      request.finished = true;
      request.unsubscribe();
      if (current === generation.current) {
        activeRequest.current = null;
        setError(String(e));
        setLoading(false);
      } else {
        toast.error(String(e));
      }
    }
  };

  return (
    <div className="space-y-2">
      {answer && (
        <div
          data-testid="command-explanation-answer"
          aria-busy={loading}
          className="select-text rounded-md border border-border bg-background/70 p-3 text-xs leading-relaxed break-words [&_p+p]:mt-2 [&_ul]:list-disc [&_ul]:pl-4 [&_ol]:list-decimal [&_ol]:pl-4 [&_code]:font-mono [&_code]:text-[11px]"
        >
          <Markdown components={markdownComponents} urlTransform={markdownUrlTransform}>
            {answer}
          </Markdown>
        </div>
      )}
      {error && (
        <p role="alert" className="text-xs text-destructive">
          {error}
        </p>
      )}
      <Button
        data-testid="explain-command"
        size="sm"
        variant="outline"
        disabled={loading}
        onClick={() => void explain()}
      >
        {loading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <BookOpen className="h-3.5 w-3.5" />}
        {t("ai.explainCommand")}
      </Button>
    </div>
  );
}
