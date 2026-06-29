import { aiRequest } from "./api-client";

export interface AIFileUploadResponse {
  file_id: string;
  original_name: string;
  content_type: string;
  object_name?: string;
}

export interface ParseDocumentRequest {
  file_id: string;
  object_name: string;
  content_type: string;
}

export const aiApi = {
  upload: (formData: FormData) =>
    aiRequest<AIFileUploadResponse>("/api/v1/files/upload", {
      method: "POST",
      body: formData,
    }),

  parse: <TResponse = unknown>(data: unknown) =>
    aiRequest<TResponse>("/api/v1/parse", {
      method: "POST",
      body: JSON.stringify(data),
    }),

  parseContract: (data: ParseDocumentRequest) =>
    aiRequest("/api/v1/parse/contract", {
      method: "POST",
      headers: { "Content-Type": "application/json" } as Record<string, string>,
      body: JSON.stringify(data),
    }),

  parsePaymentSchedule: (data: ParseDocumentRequest) =>
    aiRequest("/api/v1/parse/payment-schedule", {
      method: "POST",
      body: JSON.stringify(data),
    }),
};
