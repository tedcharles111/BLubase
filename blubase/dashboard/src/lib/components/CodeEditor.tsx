import React from 'react';

interface CodeEditorProps {
  value: string;
  onChange: (v: string) => void;
  language?: string;
  placeholder?: string;
  onRun?: () => void;
}

export default function CodeEditor({ value, onChange, language = 'sql', placeholder, onRun }: CodeEditorProps) {
  return (
    <div className="border border-zinc-200 dark:border-zinc-800 rounded-lg overflow-hidden bg-zinc-950">
      <div className="flex items-center justify-between px-4 py-2 bg-zinc-900 border-b border-zinc-800">
        <span className="text-xs font-mono text-zinc-400 uppercase">{language}</span>
        {onRun && (
          <button onClick={onRun} className="text-xs px-3 py-1 bg-emerald-600 hover:bg-emerald-700 text-white rounded font-semibold">
            Run ▶
          </button>
        )}
      </div>
      <textarea
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="w-full bg-transparent text-green-400 font-mono text-sm p-4 outline-none resize-none min-h-[200px]"
        spellCheck={false}
      />
    </div>
  );
}
