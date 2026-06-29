import { apiRequest } from "./api-client";

export const settingsApi = {
  getGlobal: (token: string) =>
    apiRequest(`/api/v1/settings/global`, { token }),

  updateGlobal: (data: { global_discount_rate: number }, token: string) =>
    apiRequest(`/api/v1/settings/global`, {
      method: "PUT",
      body: JSON.stringify(data),
      token,
    }),
};
