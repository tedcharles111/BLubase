import React, { useState, useEffect } from 'react';
import { useStore } from './lib/stores/storeHelper';
import { authStore } from './lib/stores/auth';
import { projectStore, projectActions } from './lib/stores/project';
import ToastContainer from './lib/components/Toast';
import Sidebar from './lib/components/Sidebar';
import Navbar from './lib/components/Navbar';
import AuthViews from './lib/views/AuthViews';
import ProjectsList from './lib/views/ProjectsList';
import ProjectOverview from './lib/views/ProjectOverview';
import DatabaseTablesView from './lib/views/DatabaseTablesView';
import DatabaseSQLView from './lib/views/DatabaseSQLView';
import AuthSettingsView from './lib/views/AuthSettingsView';
import StorageViews from './lib/views/StorageViews';
import EdgeFunctionsView from './lib/views/EdgeFunctionsView';
import RealtimeView from './lib/views/RealtimeView';
import LogsView from './lib/views/LogsView';
import ProjectSettingsView from './lib/views/ProjectSettingsView';
import AIAssistantView from './lib/views/AIAssistantView';

export default function App() {
  const session = useStore(authStore);
  const projState = useStore(projectStore);
  const [currentPath, setCurrentPath] = useState(window.location.pathname);

  const handleNavigate = (path: string) => {
    window.history.pushState({}, '', path);
    setCurrentPath(path);
  };

  useEffect(() => {
    const syncRoute = () => setCurrentPath(window.location.pathname);
    window.addEventListener('popstate', syncRoute);
    return () => window.removeEventListener('popstate', syncRoute);
  }, []);

  useEffect(() => {
    const isAuthRoute = currentPath.startsWith('/auth/');
    if (!session.token && !isAuthRoute) handleNavigate('/auth/signin');
    else if (session.token && (isAuthRoute || currentPath === '/')) handleNavigate('/projects');
  }, [session.token, currentPath]);

  const parseProjectRef = () => {
    const parts = currentPath.split('/');
    if (parts[1] === 'projects' && parts[2] && parts[2] !== 'new') return parts[2];
    return null;
  };

  const activeRef = parseProjectRef();
  const activeProj = projState.projects.find(p => p.ref === activeRef);

  useEffect(() => {
    if (activeRef !== projState.activeRef) projectActions.setActiveRef(activeRef);
  }, [activeRef, projState.activeRef]);

  const renderRouteContent = () => {
    if (currentPath === '/auth/signin' || currentPath === '/') return <AuthViews isSignUp={false} onNavigate={handleNavigate} />;
    if (currentPath === '/auth/signup') return <AuthViews isSignUp={true} onNavigate={handleNavigate} />;
    if (currentPath === '/projects') return <ProjectsList onNavigate={handleNavigate} />;

    if (activeRef) {
      if (!activeProj) return (
        <div className="flex flex-col items-center justify-center p-12 text-center gap-3">
          <h4 className="text-md font-bold text-red-500">Resource pool not found (404)</h4>
          <p className="text-xs text-zinc-400">Project "{activeRef}" not found.</p>
          <button onClick={() => handleNavigate('/projects')} className="text-xs font-semibold text-primary underline">Return to console</button>
        </div>
      );

      const subSegments = currentPath.split('/').slice(3).join('/');
      switch (subSegments) {
        case 'database/tables': return <DatabaseTablesView projectRef={activeRef} />;
        case 'database/sql': return <DatabaseSQLView />;
        case 'auth': return <AuthSettingsView />;
        case 'storage/files':
        case 'storage/policies': return <StorageViews />;
        case 'edge-functions': return <EdgeFunctionsView />;
        case 'realtime': return <RealtimeView />;
        case 'logs': return <LogsView />;
        case 'settings': return <ProjectSettingsView project={activeProj} />;
        case 'ai-assistant': return <AIAssistantView />;
        default: return <ProjectOverview project={activeProj} onNavigate={handleNavigate} />;
      }
    }

    return <div className="flex items-center justify-center p-12"><p className="text-zinc-500">Loading...</p></div>;
  };

  const isAuthPage = currentPath.startsWith('/auth/');
  if (isAuthPage) return (
    <div className="min-h-screen bg-white dark:bg-black text-zinc-900 dark:text-zinc-100">
      {renderRouteContent()}
      <ToastContainer />
    </div>
  );

  return (
    <div className="min-h-screen flex bg-zinc-50 dark:bg-black text-zinc-800 dark:text-zinc-100">
      <Sidebar currentPath={currentPath} onNavigate={handleNavigate} activeRef={activeRef} />
      <div className="flex-1 flex flex-col h-screen overflow-hidden">
        <Navbar />
        <main className="flex-1 overflow-y-auto p-6 md:p-8">
          <div className="max-w-7xl mx-auto h-full">
            {renderRouteContent()}
          </div>
        </main>
      </div>
      <ToastContainer />
    </div>
  );
}
