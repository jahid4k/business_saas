import React from 'react';
import { usePermission } from '@/hooks/usePermission';

interface PermissionGuardProps {
  children: React.ReactNode;
  requiredAny?: string[];
  requiredAll?: string[];
  fallback?: React.ReactNode;
}

export function PermissionGuard({
  children,
  requiredAny,
  requiredAll,
  fallback = null,
}: PermissionGuardProps) {
  const { hasAnyPermission, hasAllPermissions } = usePermission();

  const passesAny = requiredAny ? hasAnyPermission(requiredAny) : true;
  const passesAll = requiredAll ? hasAllPermissions(requiredAll) : true;

  if (passesAny && passesAll) {
    return <>{children}</>;
  }

  return <>{fallback}</>;
}
