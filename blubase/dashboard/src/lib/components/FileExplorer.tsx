import React, { useEffect, useState } from 'react';
import { toast } from './Toast';
import GlassCard from './GlassCard';
import Button from './Button';
import { Upload, Download, Trash2, HardDrive } from 'lucide-react';

interface StorageFile {
  name: string;
  size: number;
  mime_type: string;
  updated_at: string;
  url: string;
}

export default function FileExplorer() {
  const [files, setFiles] = useState<StorageFile[]>([]);
  const [bucket, setBucket] = useState('public-assets');
  const [uploadFile, setUploadFile] = useState<File | null>(null);

  useEffect(() => {
    // In a real scenario, list files via Storage API; for now we'll leave empty.
  }, [bucket]);

  const handleUpload = async () => {
    if (!uploadFile) return;
    const formData = new FormData();
    formData.append('file', uploadFile);
    try {
      const res = await fetch(`/api/storage/upload/${bucket}/${uploadFile.name}`, {
        method: 'POST',
        body: formData
      });
      if (res.ok) {
        toast.success(`File ${uploadFile.name} uploaded`);
        setUploadFile(null);
      } else {
        toast.error('Upload failed');
      }
    } catch (err) {
      toast.error('Network error');
    }
  };

  const handleDelete = async (name: string) => {
    try {
      const res = await fetch(`/api/storage/delete/${bucket}/${name}`, { method: 'DELETE' });
      if (res.ok) {
        toast.success(`Deleted ${name}`);
      } else {
        toast.error('Delete failed');
      }
    } catch (err) {
      toast.error('Network error');
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex gap-2 items-center">
        <HardDrive className="h-4 w-4" />
        <select value={bucket} onChange={e => setBucket(e.target.value)} className="p-1 text-xs border rounded">
          <option value="public-assets">public-assets</option>
          <option value="user-avatars">user-avatars</option>
          <option value="invoice-receipts">invoice-receipts</option>
        </select>
        <input type="file" onChange={e => setUploadFile(e.target.files?.[0] || null)} className="text-xs" />
        <Button variant="primary" size="sm" onClick={handleUpload}><Upload className="h-3.5 w-3.5" /> Upload</Button>
      </div>

      <GlassCard className="overflow-hidden">
        <table className="w-full text-xs">
          <thead className="bg-zinc-50 dark:bg-zinc-900">
            <tr>
              <th className="px-4 py-2 text-left">Name</th>
              <th className="px-4 py-2 text-left">Size</th>
              <th className="px-4 py-2 text-left">Type</th>
              <th className="px-4 py-2"></th>
            </tr>
          </thead>
          <tbody>
            {files.map(f => (
              <tr key={f.name} className="border-t border-zinc-200 dark:border-zinc-800">
                <td className="px-4 py-2">{f.name}</td>
                <td className="px-4 py-2">{f.size} bytes</td>
                <td className="px-4 py-2">{f.mime_type}</td>
                <td className="px-4 py-2 flex gap-2">
                  <a href={f.url} download className="text-primary hover:underline"><Download className="h-3.5 w-3.5" /></a>
                  <button onClick={() => handleDelete(f.name)} className="text-red-500"><Trash2 className="h-3.5 w-3.5" /></button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </GlassCard>
    </div>
  );
}
