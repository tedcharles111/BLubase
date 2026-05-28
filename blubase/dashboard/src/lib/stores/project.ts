import type { Project } from '../../types';

type ProjState = { projects: Project[]; activeRef: string | null };

let state: ProjState = { projects: [], activeRef: null };
const listeners = new Set<() => void>();

const update = (newState: Partial<ProjState>) => {
  state = { ...state, ...newState };
  listeners.forEach(l => l());
};

export const projectStore = {
  subscribe: (cb: () => void) => { listeners.add(cb); return () => { listeners.delete(cb); } },
  get: () => state
};

export const projectActions = {
  setActiveRef: (ref: string | null) => update({ activeRef: ref }),
  fetchProjects: async () => {
    const token = (window as any).__authToken || ''; // will be set from auth store
    const res = await fetch('/api/projects/projects', {
      headers: { 'Authorization': `Bearer ${token}` }
    });
    const data = await res.json();
    if (Array.isArray(data)) update({ projects: data });
  },
  addProject: async (name: string, region: string) => {
    const token = (window as any).__authToken || '';
    const res = await fetch('/api/projects/projects', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` },
      body: JSON.stringify({ name })
    });
    const proj = await res.json();
    if (res.ok) update({ projects: [...state.projects, proj] });
    return proj;
  },
  toggleProjectStatus: (ref: string) => {
    const projects = state.projects.map(p => p.ref === ref ? { ...p, status: p.status === 'active' ? 'paused' : 'active' } as Project : p);
    update({ projects });
  },
  deleteProject: (ref: string) => {
    update({ projects: state.projects.filter(p => p.ref !== ref) });
  }
};
