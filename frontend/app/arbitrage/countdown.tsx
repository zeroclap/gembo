"use client";

import { useState, useEffect } from "react";

export function Countdown({ targetTime }: { targetTime: string }) {
  const [remaining, setRemaining] = useState("");

  useEffect(() => {
    function update() {
      const target = new Date(targetTime).getTime();
      const now = Date.now();
      const diff = Math.max(0, Math.floor((target - now) / 1000));

      if (diff === 0) {
        setRemaining("now");
        return;
      }

      const h = Math.floor(diff / 3600);
      const m = Math.floor((diff % 3600) / 60);
      const s = diff % 60;
      setRemaining(
        `${h}h ${m.toString().padStart(2, "0")}m ${s.toString().padStart(2, "0")}s`
      );
    }

    update();
    const interval = setInterval(update, 1000);
    return () => clearInterval(interval);
  }, [targetTime]);

  return <span className="font-mono tabular-nums">{remaining}</span>;
}
