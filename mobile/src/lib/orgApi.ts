import { api } from './api';
import { setAccessToken } from './secureToken';

export const orgApi = {
  getOrganizations: async () => {
    const response = await api.get('/organizations');
    return response.data.data.organizations;
  },
  createOrganization: async (data: { name: string; slug: string }) => {
    const response = await api.post('/organizations', data);
    return response.data.data.organization;
  },
  switchOrganization: async (id: string) => {
    const response = await api.post(`/organizations/${id}/switch`);
    const { access_token } = response.data.data;
    setAccessToken(access_token);
    return response.data.data;
  },
  getMyMembership: async () => {
    const response = await api.get('/members/me');
    return response.data.data.membership;
  }
};
