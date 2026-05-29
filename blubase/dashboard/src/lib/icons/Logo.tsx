export default function Logo({ className = "h-8 w-8" }: { className?: string }) {
  return (
    <svg viewBox="0 0 32 32" className={className} fill="none" xmlns="http://www.w3.org/2000/svg">
      <circle cx="16" cy="16" r="14" fill="#2563eb" />
      <path d="M10 12 L22 12 L16 24 Z" fill="white" opacity="0.95" />
      <circle cx="16" cy="13.5" r="3" fill="white" />
      <path d="M12 19 L20 19" stroke="white" strokeWidth="1.5" strokeLinecap="round" />
      <path d="M13 22 L19 22" stroke="white" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}
