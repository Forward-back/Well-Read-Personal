import type { ReviewGroup } from '@/api/review'

export interface ReviewFlatGroup {
  group: ReviewGroup
  depth: number
  card_count: number
  due_count: number
}

export interface ReviewSourceTarget {
  knowledgeBaseId: string
  knowledgeId: string
  chunkId: string
}
