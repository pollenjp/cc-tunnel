import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { getCredentialsStatus } from '../api/credentials';
import { useAppAuth } from '../hooks/useAppAuth';

interface Props {
  conversationId: string;
  children: React.ReactNode;
}

/**
 * CredentialGuard checks whether the current user has valid credentials registered.
 * If not, it redirects to the credential login page.
 */
const CredentialGuard: React.FC<Props> = ({ conversationId, children }) => {
  const { token } = useAppAuth();
  const navigate = useNavigate();
  const [checked, setChecked] = useState(false);

  useEffect(() => {
    if (!token) return;
    getCredentialsStatus(token)
      .then(({ registered, isValid }) => {
        if (!registered) {
          navigate(
            `/login/credentials?reason=missing&conversationId=${encodeURIComponent(conversationId)}`,
            { replace: true },
          );
        } else if (!isValid) {
          navigate(
            `/login/credentials?reason=expired&conversationId=${encodeURIComponent(conversationId)}`,
            { replace: true },
          );
        } else {
          setChecked(true);
        }
      })
      .catch(() => {
        // If the endpoint is unavailable, allow children to render (fail-open).
        setChecked(true);
      });
  }, [token, conversationId, navigate]);

  if (!checked) {
    return (
      <div className="flex items-center justify-center h-screen bg-[var(--color-bg)]">
        <div className="animate-spin rounded-full h-10 w-10 border-4 border-[var(--color-accent)] border-t-transparent" />
      </div>
    );
  }

  return <>{children}</>;
};

export default CredentialGuard;
