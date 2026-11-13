export default function Button({ variant = "primary", className = "", ...props }) {
  const styles =
    variant === "primary"
      ? "bg-blue-600 text-white hover:bg-blue-700 dark:bg-blue-500 dark:text-slate-900 dark:hover:bg-blue-400"
      : "border border-slate-300 text-slate-800 hover:bg-slate-100 dark:border-slate-600 dark:text-slate-100 dark:hover:bg-slate-700";

  return (
    <button
      type="button"
      className={`mt-4 rounded-lg px-4 py-2 font-semibold disabled:cursor-not-allowed disabled:opacity-60 ${styles} ${className}`}
      {...props}
    />
  );
}
