import { Navigate, useLocation } from 'react-router-dom';
import { useAppAuth } from '../hooks/useAppAuth';

const AppAuthGuard: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { user, isLoading } = useAppAuth();
  const location = useLocation();

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-screen bg-[var(--color-bg)]">
        <div className="animate-spin rounded-full h-10 w-10 border-4 border-[var(--color-accent)] border-t-transparent" />
      </div>
    );
  }

  if (!user) {
    return <Navigate to={`/login?redirect=${location.pathname}`} replace />;
  }

  return <>{children}</>;
};

export default AppAuthGuard;
