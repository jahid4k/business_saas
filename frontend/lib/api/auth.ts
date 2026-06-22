// lib/api/auth.ts
import { apiPost } from "@/lib/api";
import type { User } from "@/types/domain";

export interface SignupInput {
  email: string;
  password: string;
  first_name: string;
  last_name: string;
}

export const authApi = {
  signup: (body: SignupInput) =>
    apiPost<{ user: User }>("api/v1/auth/signup", body),
};
