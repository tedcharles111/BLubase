import React, { useState } from 'react';
import { toast } from './Toast';
import type { ChatMessage } from '../../types';

export default function AIChatPanel() {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSend = async () => {
    if (!input.trim() || loading) return;
    const userMsg: ChatMessage = { id: 'u' + Date.now(), role: 'user', text: input, timestamp: new Date().toISOString() };
    setMessages(prev => [...prev, userMsg]);
    setInput('');
    setLoading(true);
    try {
      const res = await fetch('/api/ai/assist', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query: input, context: '' })
      });
      const data = await res.json();
      const aiMsg: ChatMessage = { id: 'a' + Date.now(), role: 'assistant', text: data.answer || 'No response', timestamp: new Date().toISOString() };
      setMessages(prev => [...prev, aiMsg]);
    } catch (err) {
      toast.error('AI call failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="flex flex-col h-[500px] border border-zinc-200 dark:border-zinc-800 rounded-lg bg-white dark:bg-zinc-950">
      <div className="flex-1 overflow-y-auto p-4 space-y-3">
        {messages.map(m => (
          <div key={m.id} className={`flex ${m.role === 'user' ? 'justify-end' : 'justify-start'}`}>
            <div className={`max-w-[80%] p-3 rounded-lg text-sm ${
              m.role === 'user' ? 'bg-primary text-white' : 'bg-zinc-100 dark:bg-zinc-900 text-zinc-800 dark:text-zinc-200'
            }`}>
              {m.text}
            </div>
          </div>
        ))}
        {loading && <div className="text-center text-zinc-400">Thinking...</div>}
      </div>
      <div className="p-3 border-t border-zinc-200 dark:border-zinc-800 flex gap-2">
        <input
          type="text"
          value={input}
          onChange={e => setInput(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && handleSend()}
          placeholder="Ask the AI assistant..."
          className="flex-1 px-3 py-2 rounded-lg border border-zinc-300 dark:border-zinc-700 bg-zinc-50 dark:bg-zinc-800 text-sm outline-none"
        />
        <button onClick={handleSend} disabled={loading} className="px-4 py-2 bg-primary text-white rounded-lg text-sm font-semibold">Send</button>
      </div>
    </div>
  );
}
