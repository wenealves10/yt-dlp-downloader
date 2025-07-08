import { useEffect } from "react";

interface Props {
  siteKey: string;
  onVerify: (token: string) => void;
}

export function Turnstile({ siteKey, onVerify }: Props) {
  useEffect(() => {
    // @ts-ignore
    if (!window.turnstile) return;

    // @ts-ignore
    const widgetId = window.turnstile.render("#turnstile-container", {
      sitekey: siteKey,
      callback: (token: string) => {
        onVerify(token);
      },
    });

    return () => {
      // @ts-ignore
      window.turnstile?.remove?.(widgetId);
    };
  }, [siteKey]);

  return <div id="turnstile-container" />;
}
