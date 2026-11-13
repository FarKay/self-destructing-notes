// Create a note. Returns { id, url, expires_at }.
export async function createNote(text, expiresInMinutes) {
  const res = await fetch("/notes", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ text, expires_in_minutes: expiresInMinutes }),
  });
  const data = await res.json();
  if (!res.ok) {
    throw new Error(data.error || "Could not create the note.");
  }
  return data;
}

// Check that a note exists WITHOUT consuming it.
// Returns { exists, expires_at }, or null if the note is gone.
export async function checkNote(id) {
  const res = await fetch(`/notes/${encodeURIComponent(id)}`);
  if (res.status === 404) return null;
  if (!res.ok) throw new Error("Could not check the note.");
  return res.json();
}

// Read a note. This destroys it. Returns the text, or null if it is gone.
export async function readNote(id) {
  const res = await fetch(`/notes/${encodeURIComponent(id)}/read`, {
    method: "POST",
  });
  if (res.status === 404) return null;
  if (!res.ok) throw new Error("Could not read the note.");
  const data = await res.json();
  return data.text;
}
