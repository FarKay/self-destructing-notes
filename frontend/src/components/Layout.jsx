export default function Layout({ children }) {
  return (
    <div className="min-h-screen bg-slate-100 px-4 py-8 text-slate-900 dark:bg-slate-900 dark:text-slate-100">
      <main className="mx-auto max-w-xl rounded-xl border border-slate-300 bg-white p-6 shadow-sm dark:border-slate-700 dark:bg-slate-800">
        <h1 className="text-2xl font-bold">Self-destructing notes</h1>
        {children}
      </main>
    </div>
  );
}
