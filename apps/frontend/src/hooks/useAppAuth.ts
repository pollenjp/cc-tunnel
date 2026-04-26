import { useContext } from 'react';
import { AppAuthContext } from '../contexts/AppAuthContext';

export const useAppAuth = () => useContext(AppAuthContext);
