export interface LegalEntityOption {
  id: string;
  code: string;
  name: string;
  country: string;
  currency: string;
}

export interface StoreOption {
  id: string;
  code: string;
  name: string;
  legal_entity_id: string;
  brand?: string | null;
  region?: string | null;
  address?: string | null;
}

export interface LandlordOption {
  id: string;
  code: string;
  name: string;
  address?: string | null;
}

export interface LegalEntityListResponse {
  legal_entities: LegalEntityOption[];
}

export interface StoreListResponse {
  stores: StoreOption[];
}

export interface LandlordListResponse {
  landlords: LandlordOption[];
}
