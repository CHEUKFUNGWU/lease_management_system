import { apiRequest } from "./api-client";
import type {
  LandlordListResponse,
  LegalEntityListResponse,
  StoreListResponse,
} from "./types/master-data";

export const legalEntityApi = {
  list: (token?: string) =>
    apiRequest<LegalEntityListResponse>("/api/v1/master-data/legal-entities", { token }),
};

export const masterDataApi = {
  listStores: (token: string, legalEntityId?: string) => {
    const qs = new URLSearchParams();
    if (legalEntityId) {
      qs.append("legal_entity_id", legalEntityId);
    }
    const queryString = qs.toString();
    return apiRequest<StoreListResponse>(
      `/api/v1/master-data/stores${queryString ? `?${queryString}` : ""}`,
      { token }
    );
  },

  listLandlords: (token: string) =>
    apiRequest<LandlordListResponse>("/api/v1/master-data/landlords", { token }),
};
