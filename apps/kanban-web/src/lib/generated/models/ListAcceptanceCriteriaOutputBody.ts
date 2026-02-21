/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { AcceptanceCriterion } from './AcceptanceCriterion';
export type ListAcceptanceCriteriaOutputBody = {
    /**
     * A URL to the JSON Schema for this object.
     */
    readonly $schema?: string;
    acceptance_criteria: Array<AcceptanceCriterion>;
};

