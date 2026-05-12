from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

app = FastAPI(
    title="IFRS 16 AI Service",
    description="AI document parsing and field extraction service",
    version="0.1.0"
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

@app.get("/health")
async def health_check():
    return {"status": "ok", "service": "ai-service"}

@app.post("/api/v1/parse")
async def parse_document():
    """Parse uploaded document and extract fields"""
    return {"message": "Document parsing endpoint"}
