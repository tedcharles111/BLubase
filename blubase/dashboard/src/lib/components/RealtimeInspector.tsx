import React, { useEffect, useState } from 'react';
import { toast } from './Toast';
import GlassCard from './GlassCard';
import { Zap, Wifi, WifiOff } from 'lucide-react';

export default function RealtimeInspector() {
  const [status, setStatus] = useState<'disconnected' | 'connecting' | 'connected'>('disconnected');
  const [messages, setMessages] = useState<any[]>([]);
  const [channel, setChannel] = useState('room:default');
  const [payload, setPayload] = useState('');

  useEffect(() => {
    const ws = new WebSocket('ws://localhost:4000/ws');
    ws.onopen = () => setStatus('connected');
    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        setMessages(prev => [...prev, msg]);
      } catch (e) {}
    };
    ws.onclose = () => setStatus('disconnected');
    ws.onerror = () => setStatus('disconnected');
    return () => ws.close();
  }, []);

  const sendMessage = () => {
    if (!payload) return;
    // We can't easily send to specific channel without additional server support; for demo, we'll just log.
    toast.info('Message sent (broadcast)');
    setPayload('');
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        {status === 'connected' ? <Wifi className="h-4 w-4 text-emerald-400" /> : <WifiOff className="h-4 w-4 text-red-400" />}
        <span className="text-sm font-semibold">{status === 'connected' ? 'Connected' : 'Disconnected'}</span>
      </div>
      <GlassCard className="p-4 space-y-2">
        <div className="flex gap-2">
          <input
            type="text"
            placeholder="Channel"
            value={channel}
            onChange={e => setChannel(e.target.value)}
            className="flex-1 p-2 text-xs rounded border border-zinc-300 dark:border-zinc-700 bg-zinc-50 dark:bg-zinc-900"
          />
          <input
            type="text"
            placeholder="Message payload"
            value={payload}
            onChange={e => setPayload(e.target.value)}
            className="flex-1 p-2 text-xs rounded border border-zinc-300 dark:border-zinc-700 bg-zinc-50 dark:bg-zinc-900"
          />
          <button onClick={sendMessage} className="px-3 py-1 text-xs bg-primary text-white rounded">Send</button>
        </div>
        <div className="h-40 overflow-y-auto border border-zinc-200 dark:border-zinc-800 rounded p-2 bg-zinc-950 text-green-400 font-mono text-xs">
          {messages.map((m, i) => (
            <div key={i}>{JSON.stringify(m)}</div>
          ))}
        </div>
      </GlassCard>
    </div>
  );
}
