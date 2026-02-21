/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { AcceptanceCriterion } from './AcceptanceCriterion';
import type { HistoryEvent } from './HistoryEvent';
import type { TextEvent } from './TextEvent';
import type { Todo } from './Todo';
export type Card = {
    /**
     * A URL to the JSON Schema for this object.
     */
    readonly $schema?: string;
    acceptance_criteria: Array<AcceptanceCriterion>;
    branch: string;
    comments: Array<TextEvent>;
    created_at: string;
    deleted: boolean;
    description: Array<TextEvent>;
    history: Array<HistoryEvent>;
    id: string;
    number: number;
    project: string;
    status: string;
    title: string;
    todos: Array<Todo>;
    updated_at: string;
};

