import _ from 'lodash';
import { PartialDeep } from 'type-fest';
import { User } from '@/types/user';

/**
 * Creates a normalized application user object.
 *
 * This keeps the original Fuse user shape and adds safe SaaS defaults.
 */
function UserModel(data?: PartialDeep<User>): User {
    data = data || {};

    return _.defaults(data, {
        id: null,
        role: null,
        displayName: null,
        photoURL: '',
        email: '',
        shortcuts: [],
        settings: {},
        loginRedirectUrl: '/',
        organizationId: null,
        organizationRole: null,
        permissions: [],
        plan: 'free',
        status: 'active',
        emailVerified: false
    }) as User;
}

export default UserModel;
