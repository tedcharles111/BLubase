export default function EdgeIcon({ className = "h-5 w-5" }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" className={className} fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="4" y="4" width="16" height="16" rx="2" />
      <rect x="9" y="9" width="6" height="6" />
      <line x1="9" y1="4" x2="9" y2="6" />
      <line x1="15" y1="4" x2="15" y2="6" />
      <line x1="9" y1="18" x2="9" y2="20" />
      <line x1="15" y1="18" x2="15" y2="20" />
      <line x1="4" y1="9" x2="6" y2="9" />
      <line x1="4" y1="15" x2="6" y2="15" />
      <line x1="18" y1="9" x2="20" y2="9" />
      <line x1="18" y1="15" x2="20" y2="15" />
    </svg>
  );
}
