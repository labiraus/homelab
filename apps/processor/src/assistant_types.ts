export interface ConversationRecord {
	id: number;
	conversationId: string;
	userEmail: string;
	title: string;
	status: string;
	createdAt: Date;
	updatedAt: Date;
}

export interface MessageRecord {
	messageId: string;
	conversationId: string;
	role: string;
	content: string;
	citations: unknown[];
	metadata: Record<string, unknown>;
	createdAt: Date;
}

export interface MemoryRecord {
	memoryId: string;
	userEmail: string;
	text: string;
	sourceConversationId?: string;
	status: string;
	createdAt: Date;
	archivedAt?: string;
}

export interface FileProposalRecord {
	proposalId: string;
	conversationId: string;
	userEmail: string;
	action: string;
	documentId?: string;
	objectKey: string;
	contentType: string;
	proposedText: string;
	rationale: string;
	status: string;
	orchestratorResponse?: Record<string, unknown>;
	createdAt: Date;
	decidedAt?: string;
}

export interface ToolCallRecord {
	toolCallId?: string;
	conversationId?: string;
	messageId?: string;
	userEmail: string;
	toolName: string;
	arguments: Record<string, unknown>;
	result: Record<string, unknown>;
	isError: boolean;
	createdAt?: Date;
}

export interface AuditRecord {
	auditId: string;
	documentId: string;
	objectKey: string;
	action: string;
	actorEmail: string;
	conversationId?: string;
	proposalId?: string;
	oldVersionMarker?: string;
	newVersionMarker?: string;
	revertedToVersionMarker?: string;
	processingVersion: number;
	metadata: Record<string, unknown>;
	createdAt: Date;
}

export interface ChatRequest {
	conversationId?: string;
	message: string;
	title?: string;
}

export interface ChatResponse {
	conversation: ConversationRecord;
	userMessage: MessageRecord;
	reply: MessageRecord;
	toolCalls: ToolCallRecord[];
}

export interface CreateConversationRequest {
	title?: string;
}

export interface CreateMemoryRequest {
	text: string;
	sourceConversationId?: string;
}

export interface CreateProposalRequest {
	conversationId: string;
	action: string;
	documentId?: string;
	objectKey: string;
	contentType?: string;
	proposedText: string;
	rationale?: string;
}

export interface AssistantRunResult {
	content: string;
	citations: unknown[];
	metadata: Record<string, unknown>;
	toolCalls: ToolCallRecord[];
}

export interface DocumentContextResult {
	context: string;
	citations: unknown[];
	raw: Record<string, unknown>;
	isError: boolean;
	error?: string;
}
