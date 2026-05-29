export default function AIIcon({ className = "h-5 w-5" }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" className={className} fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 2l2.5 6.5L21 9l-6.5 2.5L12 18l-2.5-6.5L3 9l6.5-2.5L12 2z" />
      <path d="M12 18v4" />
      <path d="M8 22h8" />
    </svg>
  );
}
