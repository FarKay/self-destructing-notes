import { useState } from "react";
import { createNote } from "../api";
import Button from "../components/Button";

const EXPIRY_OPTIONS = [
  { label: "10 minutes", value: 10 },
  { label: "1 hour", value: 60 },
  { label: "1 day", value: 1440 },
  { label: "7 days", value: 10080 },
];

const inputClasses =
  "mt-1 w-full rounded-lg border border-slate-300 bg-slate-50 p-2.5 text-slate-900 dark:border-slate-600 dark:bg-slate-900 dark:text-slate-100";
const mutedClasses = "text-slate-600 dark:text-slate-400";
const errorClasses = "mt-3 font-semibold text-red-700 dark:text-red-400";

export default function CreatePage() {
  const [text, setText] = useState("");
  const [expiry, setExpiry] = useState(60);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState(null); // { link, expiresAt } once created
  const [copied, setCopied] = useState(false);

  async function handleCreate() {
    setError("");
    if (text.trim() === "") {
      setError("Please write something first.");
      return;
    }

    setLoading(true);
    try {
      const data = await createNote(text, expiry);
      setResult({
        // build the link from the address the person is actually using
        link: `${window.location.origin}/n/${data.id}`,
        expiresAt: new Date(data.expires_at).toLocaleString(),
      });
      setText(""); // do not keep the secret in the page
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(result.link);
      setCopied(true);
    } catch {
      // clipboard blocked: the person can select and copy the link by hand
    }
  }

  function handleReset() {
    setResult(null);
    setCopied(false);
  }

  if (result) {
    return (
      <div>
        <p className="mt-4">Your link. It works only once:</p>
        <input
          type="text"
          readOnly
          value={result.link}
          onFocus={(e) => e.target.select()}
          className={inputClasses}
        />
        <div className="flex flex-wrap gap-3">
          <Button onClick={handleCopy}>{copied ? "Copied!" : "Copy link"}</Button>
          <Button variant="secondary" onClick={handleReset}>
            Create another
          </Button>
        </div>
        <p className={`mt-4 text-sm ${mutedClasses}`}>Expires: {result.expiresAt}</p>
      </div>
    );
  }

  return (
    <div>
      <p className={mutedClasses}>
        Write a secret. Share the link. It can be read once, then it is gone.
      </p>

      <label htmlFor="text" className="mt-4 block font-semibold">
        Your note
      </label>
      <textarea
        id="text"
        rows={6}
        maxLength={10000}
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder="Type something secret..."
        className={`${inputClasses} resize-y`}
      />

      <label htmlFor="expiry" className="mt-4 block font-semibold">
        Destroy after
      </label>
      <select
        id="expiry"
        value={expiry}
        onChange={(e) => setExpiry(Number(e.target.value))}
        className={inputClasses}
      >
        {EXPIRY_OPTIONS.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>

      <Button onClick={handleCreate} disabled={loading}>
        {loading ? "Creating..." : "Create link"}
      </Button>

      {error && <p className={errorClasses}>{error}</p>}
    </div>
  );
}
