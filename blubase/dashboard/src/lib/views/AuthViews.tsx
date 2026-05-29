import React, { useState } from 'react';
import { toast } from '../components/Toast';
import { authActions } from '../stores/auth';
import Button from '../components/Button';
import Logo from '../icons/Logo';

export default function AuthViews({ isSignUp, onNavigate }: { isSignUp: boolean; onNavigate: (p: string) => void }) {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      if (isSignUp) {
        const data = await authActions.signup(email, password);
        if (data.message || data.token) {
          toast.success('Account created! You can now sign in.');
          onNavigate('/auth/signin');
        } else {
          toast.error(data.error || 'Signup failed');
        }
      } else {
        const data = await authActions.login(email, password);
        if (data.token) {
          toast.success('Welcome back!');
          onNavigate('/projects');
        } else {
          toast.error(data.error || 'Invalid credentials');
        }
      }
    } catch (err) {
      toast.error('Network error – check backend connection');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen w-full flex flex-col md:flex-row">
      {/* Left Panel – Visual Identity */}
      <div className="hidden md:flex md:w-5/12 bg-black flex-col justify-center items-center p-12 text-white relative overflow-hidden">
        <div className="absolute inset-0 bg-gradient-to-br from-blue-600/20 to-purple-600/10" />
        <div className="relative z-10 space-y-8 text-center">
          <Logo className="h-20 w-20 mx-auto drop-shadow-lg" />
          <h1 className="text-4xl font-bold font-display tracking-tight">Blubase</h1>
          <p className="text-lg text-blue-100/80 font-sans max-w-xs leading-relaxed">
            The open‑source backend that scales.
            <br />Postgres, Auth, Storage, Edge, AI – all in one.
          </p>
          <div className="flex gap-6 justify-center text-xs font-mono text-blue-200/60">
            <span>Auth 3001</span>
            <span>Edge 3005</span>
            <span>AI 3006</span>
          </div>
        </div>
      </div>

      {/* Right Panel – Form */}
      <div className="flex-1 flex items-center justify-center p-6 bg-white dark:bg-zinc-950">
        <div className="w-full max-w-md space-y-6">
          <div className="mb-6 md:hidden text-center">
            <Logo className="h-12 w-12 mx-auto" />
            <h1 className="text-2xl font-bold mt-2 font-display">Blubase</h1>
          </div>

          <h2 className="text-2xl font-bold tracking-tight font-display">
            {isSignUp ? 'Create your account' : 'Sign in to Blubase'}
          </h2>
          <p className="text-sm text-zinc-500">
            {isSignUp ? 'Start building with a free project.' : 'Enter your credentials to continue.'}
          </p>

          <form onSubmit={handleSubmit} className="space-y-5">
            <div>
              <label className="block text-sm font-medium mb-1">Email</label>
              <input
                type="email"
                value={email}
                onChange={e => setEmail(e.target.value)}
                className="w-full p-3 rounded-lg border border-zinc-300 dark:border-zinc-700 bg-zinc-50 dark:bg-zinc-900 focus:ring-2 focus:ring-primary focus:border-transparent outline-none transition"
                required
              />
            </div>
            <div>
              <label className="block text-sm font-medium mb-1">Password</label>
              <input
                type="password"
                value={password}
                onChange={e => setPassword(e.target.value)}
                className="w-full p-3 rounded-lg border border-zinc-300 dark:border-zinc-700 bg-zinc-50 dark:bg-zinc-900 focus:ring-2 focus:ring-primary focus:border-transparent outline-none transition"
                required
              />
            </div>
            <Button type="submit" variant="primary" size="lg" loading={loading} className="w-full">
              {isSignUp ? 'Sign up' : 'Log in'}
            </Button>
          </form>

          <div className="text-sm text-zinc-500 text-center">
            {isSignUp ? 'Already have an account?' : "Don't have an account?"}{' '}
            <button
              onClick={() => onNavigate(isSignUp ? '/auth/signin' : '/auth/signup')}
              className="text-primary font-semibold hover:underline"
            >
              {isSignUp ? 'Sign in' : 'Sign up'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
