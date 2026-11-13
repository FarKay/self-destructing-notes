import { useEffect, useState } from "react";
import { checkNote, readNote } from "../api";
import Button from "../components/Button";

const mutedClasses = "text-slate-600 dark:text-slate-400";
const errorClasses = "mt-3 font-semibold text-red-700 dark:text-red-400";

export default function NotePage({ id }) {
  // status: "loading" | "ready" | "revealed" | "gone"
  const [status, setStatus] = useState("loading");
  const [expiresAt, setExpiresAt] = useState("");
  const [text, setText] = useState("");
  const [error, setError] = useState("");
  const [revealing, setRevealing] = useState(false);
  const [copied, setCopied] = useState(false);

  // On load we only CHECK the note. Reading it must stay behind the button:
  // never put readNote in an effect, or opening the page would destroy it.
  useEffect(() => {
    let cancelled = false;

    async function check() {
      try {
        const note = await checkNote(id);
        if (cancelled) return;
        if (note === null) {
          setStatus("gone");
          return;
        }
        setExpiresAt(new Date(note.expires_at).toLocaleString());
        setStatus("ready");
      } catch {
        if (!cancelled) setError("Could not reach the server. Try again.");
      }
    }

    check();
    return () => {
      cancelled = true;
    };
  }, [id]);

  async function handleReveal() {
    setRevealing(true);
    setError("");
    try {
      const noteText = await readNote(id);
      if (noteText === null) {
        setStatus("gone");
        return;
      }
      setText(noteText);
      setStatus("revealed");
    } catch {
      setError("Could not reach the server. Try again.");
    } finally {
      setRevealing(false);
    }
  }

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
    } catch {
      setError("Could not copy. Select the text and copy it by hand.");
    }
  }

  return (
    <div>
      {status === "loading" && !error && <p className="mt-4">Checking the note...</p>}

      {status === "ready" && (
        <>
          <p className="mt-4">
            Someone sent you a secret note. It can be read <strong>once</strong>.
            After you reveal it, it is destroyed.
          </p>
          <p className={`mt-2 text-sm ${mutedClasses}`}>Expires: {expiresAt}</p>
          <Button onClick={handleReveal} disabled={revealing}>
            {revealing ? "Revealing..." : "Reveal note"}
          </Button>
        </>
      )}

      {status === "revealed" && (
        <>
          <p className="mt-4">This is the only time you will see this note:</p>
          {/* React shows this as plain text, never as HTML, so a note cannot inject code */}
          <pre className="mt-2 whitespace-pre-wrap break-words rounded-lg border border-slate-300 bg-slate-50 p-3 font-mono dark:border-slate-600 dark:bg-slate-900">
            {text}
          </pre>
          <Button variant="secondary" onClick={handleCopy}>
            {copied ? "Copied!" : "Copy text"}
          </Button>
        </>
      )}

      {status === "gone" && (
        <p className="mt-4">
          This note does not exist, has expired, or has already been read.
        </p>
      )}

      {error && <p className={errorClasses}>{error}</p>}

      <p className={`mt-6 text-sm ${mutedClasses}`}>
        <a href="/" className="text-blue-700 underline dark:text-blue-400">
          Create your own note
        </a>
      </p>
    </div>
  );
}
