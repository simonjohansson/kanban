/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { WebsocketEventType } from './WebsocketEventType';
export type WebsocketEvent = {
    card_id?: string;
    card_number?: number;
    project: string;
    timestamp: string;
    type: WebsocketEventType;
};

