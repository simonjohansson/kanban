/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { HistoryEvent } from './HistoryEvent';
import type { TextEvent } from './TextEvent';
export type Card = {
    /**
     * A URL to the JSON Schema for this object.
     */
    readonly $schema?: string;
    branch?: string;
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
    updated_at: string;
};

