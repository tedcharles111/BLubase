import React, { useState } from 'react';
import { toast } from '../components/Toast';
import GlassCard from '../components/GlassCard';
import CodeEditor from '../components/CodeEditor';

export default function DatabaseSQLView() {
  const [sqlQuery, setSqlQuery] = useState('SELECT * FROM user_profiles LIMIT 10;');
  const [executing, setExecuting] = useState(false);
  const [results, setResults] = useState<any[] | null>(null);
  const [columns, setColumns] = useState<string[]>([]);
  const [errorText, setErrorText] = useState<string | null>(null);

  const handleRunQuery = async () => {
    setExecuting(true);
    setErrorText(null);
    setResults(null);
    try {
      const res = await fetch(`/api/sql/sql?query=${encodeURIComponent(sqlQuery)}`);
      const data = await res.json();
      if (!res.ok) throw new Error(data?.error || 'Query failed');
      if (Array.isArray(data)) {
        if (data.length > 0) {
          setColumns(Object.keys(data[0]));
          setResults(data);
          toast.success(`Retrieved ${data.length} records`);
        } else {
          setColumns([]);
          setResults([]);
          toast.success('Query executed, 0 rows returned.');
        }
      } else {
        setResults([]);
        setColumns([]);
        toast.success('Command executed.');
      }
    } catch (err) {
      setErrorText((err as Error).message);
      toast.error((err as Error).message);
    } finally {
      setExecuting(false);
    }
  };

  return (
    <div className="space-y-4">
      <CodeEditor value={sqlQuery} onChange={setSqlQuery} language="sql" onRun={handleRunQuery} />
      <GlassCard className="p-4 min-h-[150px]">
        <h3 className="text-xs font-semibold text-zinc-500 uppercase mb-2">Execution Output</h3>
        {executing && <div className="text-zinc-400 animate-pulse">Running query...</div>}
        {errorText && <div className="text-red-500 text-xs">{errorText}</div>}
        {!executing && !errorText && results && results.length > 0 && (
          <table className="w-full text-xs">
            <thead><tr>{columns.map(c => <th key={c} className="text-left p-1">{c}</th>)}</tr></thead>
            <tbody>{results.map((row, i) => <tr key={i}>{columns.map(c => <td key={c} className="p-1">{row[c]}</td>)}</tr>)}</tbody>
          </table>
        )}
        {!executing && !errorText && results && results.length === 0 && <div className="text-zinc-400">No rows returned.</div>}
        {!executing && !errorText && !results && <div className="text-zinc-400">Write a query and click Run.</div>}
      </GlassCard>
    </div>
  );
}
