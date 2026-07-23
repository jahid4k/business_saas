import { useState } from 'react';
import { orgApi } from '@/lib/orgApi';
import { useAuthStore } from '@/stores/authStore';
import { usePermissionStore } from '@/stores/permissionStore';
import { Organization, MembershipWithRole } from '@/types';
import { useRouter } from 'expo-router';

export function useOrganization() {
  const [loading, setLoading] = useState(false);
  const { setOrg } = useAuthStore();
  const { setPermissions } = usePermissionStore();
  const router = useRouter();

  const getOrganizations = async () => {
    setLoading(true);
    try {
      const response = await orgApi.getOrganizations();
      return response as MembershipWithRole[];
    } finally {
      setLoading(false);
    }
  };

  const createOrganization = async (data: { name: string; slug: string }) => {
    setLoading(true);
    try {
      const response = await orgApi.createOrganization(data);
      return response as Organization;
    } finally {
      setLoading(false);
    }
  };

  const switchOrg = async (org: Organization) => {
    setLoading(true);
    try {
      // The backend expects the org ID in the URL. If the type is Business, it might be publicId.
      // We'll use org.id, assuming it maps correctly.
      await orgApi.switchOrganization(org.id || (org as any).publicId);

      const membershipData = await orgApi.getMyMembership();

      setPermissions(membershipData.permissions || []);
      setOrg(org, {
        membershipId: membershipData.membershipId,
        organizationId: membershipData.organizationId,
        role: membershipData.role,
        joinedAt: membershipData.joinedAt,
      });

      router.push(`/(dashboard)/${org.id || (org as any).publicId}`);
    } finally {
      setLoading(false);
    }
  };

  return {
    getOrganizations,
    createOrganization,
    switchOrg,
    loading,
  };
}
