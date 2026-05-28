import React from 'react';

type ButtonProps = {
  variant?: 'primary' | 'success' | 'warning' | 'danger' | 'outline';
  size?: 'sm' | 'md' | 'lg';
  loading?: boolean;
  children: React.ReactNode;
} & React.ButtonHTMLAttributes<HTMLButtonElement>;

export default function Button({ variant = 'primary', size = 'md', loading, children, ...props }: ButtonProps) {
  const base = "inline-flex items-center gap-1.5 font-medium rounded-lg transition-all duration-150 disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer";
  const variants = {
    primary: 'bg-primary text-white hover:bg-blue-600',
    success: 'bg-emerald-600 text-white hover:bg-emerald-700',
    warning: 'bg-amber-500 text-white hover:bg-amber-600',
    danger: 'bg-red-500 text-white hover:bg-red-600',
    outline: 'border border-zinc-300 dark:border-zinc-700 bg-transparent text-zinc-700 dark:text-zinc-300 hover:bg-zinc-100 dark:hover:bg-zinc-800'
  };
  const sizes = {
    sm: 'px-3 py-1.5 text-xs',
    md: 'px-4 py-2 text-sm',
    lg: 'px-6 py-3 text-base'
  };
  return (
    <button className={`${base} ${variants[variant]} ${sizes[size]}`} disabled={loading} {...props}>
      {loading && <span className="h-4 w-4 border-2 border-white border-t-transparent rounded-full animate-spin mr-1" />}
      {children}
    </button>
  );
}
