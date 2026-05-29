import React, { useEffect, useState } from 'react';
import { DatabaseTable } from '../../types';
import { toast } from '../components/Toast';
import { Database, Plus, Search } from 'lucide-react';
import GlassCard from '../components/GlassCard';
import TableViewer from '../components/TableViewer';

export default function DatabaseTablesView({ projectRef }: { projectRef: string }) {
  const [tables, setTables] = useState<DatabaseTable[]>([]);
  const [activeTable, setActiveTable] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [showAddTable, setShowAddTable] = useState(false);
  const [newTableName, setNewTableName] = useState('');

  // Fetch tables from backend
  useEffect(() => {
    (async () => {
      try {
        // Using SQL endpoint to list tables (query the information_schema)
        const res = await fetch(`/api/sql/sql?query=${encodeURIComponent(
          `SELECT table_name FROM information_schema.tables WHERE table_schema='public' ORDER BY table_name`
        )}`);
        const data = await res.json();
        if (Array.isArray(data)) {
          const tableNames: string[] = data.map((r: any) => r.table_name);
          // For each table, fetch columns
          const tableDetails: DatabaseTable[] = [];
          for (const name of tableNames) {
            const colsRes = await fetch(`/api/sql/sql?query=${encodeURIComponent(
              `SELECT column_name, data_type, is_nullable, column_default FROM information_schema.columns WHERE table_name='${name}' ORDER BY ordinal_position`
            )}`);
            const colsData = await colsRes.json();
            const columns = Array.isArray(colsData) ? colsData.map((c: any) => ({
              name: c.column_name,
              type: c.data_type,
              is_nullable: c.is_nullable === 'YES',
              is_primary: false, // we could check constraints but keep simple
              default_value: c.column_default
            })) : [];
            // Fetch rows (limit 5)
            const rowsRes = await fetch(`/api/sql/sql?query=${encodeURIComponent(`SELECT * FROM ${name} LIMIT 5`)}`);
            const rowsData = await rowsRes.json();
            tableDetails.push({
              name,
              schema: 'public',
              rows_count: Array.isArray(rowsData) ? rowsData.length : 0,
              columns,
              rows: Array.isArray(rowsData) ? rowsData : []
            });
          }
          setTables(tableDetails);
          if (tableDetails.length > 0) setActiveTable(tableDetails[0].name);
        }
      } catch (e) {
        console.error('Failed to fetch tables:', e);
        toast.error('Could not load database tables.');
      }
    })();
  }, [projectRef]);

  const handleCreateTable = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newTableName) return;
    try {
      await fetch(`/api/sql/sql?query=${encodeURIComponent(`CREATE TABLE ${newTableName} (id UUID PRIMARY KEY DEFAULT gen_random_uuid(), created_at TIMESTAMPTZ DEFAULT NOW())`)}`);
      toast.success(`Table ${newTableName} created!`);
      setShowAddTable(false);
      setNewTableName('');
      // Refresh (simple reload of window, better to refetch)
      window.location.reload();
    } catch (err) {
      toast.error('Failed to create table');
    }
  };

  const currentTable = tables.find(t => t.name === activeTable);

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-4">
        <div className="relative">
          <Search className="absolute left-2 top-1/2 -translate-y-1/2 h-4 w-4 text-zinc-400" />
          <input
            type="text"
            placeholder="Search tables..."
            value={search}
            onChange={e => setSearch(e.target.value)}
            className="pl-8 pr-4 py-2 text-sm rounded-lg border border-zinc-300 dark:border-zinc-700 bg-white dark:bg-zinc-900"
          />
        </div>
        <button onClick={() => setShowAddTable(true)} className="flex items-center gap-1 text-sm font-semibold text-primary hover:underline">
          <Plus className="h-4 w-4" /> New Table
        </button>
      </div>

      {showAddTable && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <GlassCard className="p-6 w-80">
            <h3 className="font-bold mb-4">Create Table</h3>
            <form onSubmit={handleCreateTable}>
              <input
                value={newTableName}
                onChange={e => setNewTableName(e.target.value)}
                className="w-full p-2 border rounded mb-4"
                placeholder="table_name"
                required
              />
              <div className="flex justify-end gap-2">
                <button type="button" onClick={() => setShowAddTable(false)} className="text-sm">Cancel</button>
                <button type="submit" className="text-sm bg-primary text-white px-3 py-1 rounded">Create</button>
              </div>
            </form>
          </GlassCard>
        </div>
      )}

      <div className="flex gap-4">
        <div className="w-48 space-y-1">
          {tables.filter(t => t.name.includes(search)).map(t => (
            <div
              key={t.name}
              onClick={() => setActiveTable(t.name)}
              className={`p-2 rounded cursor-pointer text-sm ${
                activeTable === t.name ? 'bg-primary text-white' : 'hover:bg-zinc-100 dark:hover:bg-zinc-800'
              }`}
            >
              {t.name}
            </div>
          ))}
        </div>
        <div className="flex-1">
          {currentTable && (
            <TableViewer table={currentTable} onUpdateTable={(updated) => {
              setTables(tables.map(t => t.name === updated.name ? updated : t));
            }} />
          )}
        </div>
      </div>
    </div>
  );
}
