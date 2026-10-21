import { useEffect } from "react";
import { useBlocker } from "react-router-dom";

/** Warn on in-app navigation and browser unload when dirty. */
export function useUnsavedChanges(dirty: boolean) {
  const blocker = useBlocker(dirty);

  useEffect(() => {
    if (!dirty) return;
    function onBeforeUnload(e: BeforeUnloadEvent) {
      e.preventDefault();
      e.returnValue = "";
    }
    window.addEventListener("beforeunload", onBeforeUnload);
    return () => window.removeEventListener("beforeunload", onBeforeUnload);
  }, [dirty]);

  useEffect(() => {
    if (blocker.state !== "blocked") return;
    const ok = window.confirm("You have unsaved changes. Leave this page?");
    if (ok) blocker.proceed();
    else blocker.reset();
  }, [blocker]);
}
