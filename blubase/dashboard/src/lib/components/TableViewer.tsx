import React, { useState } from 'react';
import { DatabaseTable } from '../../types';
import { Plus, Trash2 } from 'lucide-react';
import GlassCard from './GlassCard';
import Button from './Button';
import { toast } from './Toast';

interface TableViewerProps {
  table: DatabaseTable;
  onUpdateTable: (updated: DatabaseTable) => void;
}

export default function TableViewer({ table, onUpdateTable }: TableViewerProps) {
  const [editingRow, setEditingRow] = useState<Record<string, any> | null>(null);
  const [showAddRow, setShowAddRow] = useState(false);
  const [newRowData, setNewRowData] = useState<Record<string, any>>({});

  const handleDeleteRow = async (rowIndex: number) => {
    // SQL to delete row using primary key (assuming id column)
    const row = table.rows[rowIndex];
    if (!row || !row.id) return toast.error('Cannot delete row without primary key');
    try {
      await fetch(`/api/sql/sql?query=${encodeURIComponent(`DELETE FROM ${table.name} WHERE id='${row.id}'`)}`);
      const updatedRows = table.rows.filter((_, i) => i !== rowIndex);
      onUpdateTable({ ...table, rows: updatedRows, rows_count: updatedRows.length });
      toast.success('Row deleted');
    } catch (err) {
      toast.error('Delete failed');
    }
  };

  const handleAddRow = async () => {
    const columns = table.columns.map(c => c.name).join(', ');
    const values = table.columns.map(c => {
      const val = newRowData[c.name];
      if (val === undefined || val === '') return 'NULL';
      return `'${val}'`;
    }).join(', ');
    const query = `INSERT INTO ${table.name} (${columns}) VALUES (${values})`;
    try {
      await fetch(`/api/sql/sql?query=${encodeURIComponent(query)}`);
      toast.success('Row inserted');
      // Refresh data: we'll cheat and add locally
      const newRow = { ...newRowData };
      table.columns.forEach(c => {
        if (c.name === 'id') newRow.id = 'generated';
      });
      onUpdateTable({ ...table, rows: [...table.rows, newRow], rows_count: table.rows.length + 1 });
      setShowAddRow(false);
      setNewRowData({});
    } catch (err) {
      toast.error('Insert failed');
    }
  };

  return (
    <div>
      <div className="flex justify-between items-center mb-2">
        <h3 className="text-sm font-bold">{table.name}</h3>
        <Button variant="primary" size="sm" onClick={() => setShowAddRow(true)}><Plus className="h-3.5 w-3.5" /> Add Row</Button>
      </div>

      {showAddRow && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <GlassCard className="p-6 w-96">
            <h3 className="font-bold mb-4">New Row</h3>
            {table.columns.map(col => (
              <div key={col.name} className="mb-2">
                <label className="text-xs font-semibold">{col.name}</label>
                <input
                  className="w-full p-1 text-xs border rounded"
                  onChange={e => setNewRowData({ ...newRowData, [col.name]: e.target.value })}
                />
              </div>
            ))}
            <div className="flex justify-end gap-2 mt-4">
              <button onClick={() => setShowAddRow(false)} className="text-sm">Cancel</button>
              <button onClick={handleAddRow} className="text-sm bg-primary text-white px-3 py-1 rounded">Insert</button>
            </div>
          </GlassCard>
        </div>
      )}

      <GlassCard className="overflow-x-auto">
        <table className="w-full text-xs">
          <thead>
            <tr className="bg-zinc-50 dark:bg-zinc-900">
              {table.columns.map(c => (
                <th key={c.name} className="px-2 py-1 text-left">{c.name}</th>
              ))}
              <th className="px-2 py-1"></th>
            </tr>
          </thead>
          <tbody>
            {table.rows.map((row, i) => (
              <tr key={i} className="border-t border-zinc-200 dark:border-zinc-800">
                {table.columns.map(c => (
                  <td key={c.name} className="px-2 py-1">{row[c.name]}</td>
                ))}
                <td className="px-2 py-1">
                  <button onClick={() => handleDeleteRow(i)} className="text-red-500 hover:underline">Del</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </GlassCard>
    </div>
  );
}
