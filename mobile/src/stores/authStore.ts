import { create } from 'zustand';
import { User, Organization, Membership } from '@/types';

type AuthStatus = 'idle' | 'loading' | 'authenticated' | 'unauthenticated';

interface AuthStore {
  user: User | null;
  currentOrg: Organization | null;
  membership: Membership | null;
  status: AuthStatus;
  setUser: (user: User | null) => void;
  setOrg: (org: Organization | null, membership: Membership | null) => void;
  setStatus: (status: AuthStatus) => void;
  reset: () => void;
}

export const useAuthStore = create<AuthStore>((set) => ({
  user: null,
  currentOrg: null,
  membership: null,
  status: 'idle',
  setUser: (user) => set({ user }),
  setOrg: (currentOrg, membership) => set({ currentOrg, membership }),
  setStatus: (status) => set({ status }),
  reset: () => set({ user: null, currentOrg: null, membership: null, status: 'unauthenticated' }),
}));
