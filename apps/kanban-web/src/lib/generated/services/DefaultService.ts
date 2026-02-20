/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { Card } from '../models/Card';
import type { ClientConfigOutputBody } from '../models/ClientConfigOutputBody';
import type { CreateCardRequest } from '../models/CreateCardRequest';
import type { CreateProjectRequest } from '../models/CreateProjectRequest';
import type { DeleteProjectOutputBody } from '../models/DeleteProjectOutputBody';
import type { ErrorModel } from '../models/ErrorModel';
import type { HealthOutputBody } from '../models/HealthOutputBody';
import type { ListCardsOutputBody } from '../models/ListCardsOutputBody';
import type { ListProjectsOutputBody } from '../models/ListProjectsOutputBody';
import type { MoveCardRequest } from '../models/MoveCardRequest';
import type { Project } from '../models/Project';
import type { RebuildProjectionOutputBody } from '../models/RebuildProjectionOutputBody';
import type { SetCardBranchRequest } from '../models/SetCardBranchRequest';
import type { TextBodyRequest } from '../models/TextBodyRequest';
import type { CancelablePromise } from '../core/CancelablePromise';
import { OpenAPI } from '../core/OpenAPI';
import { request as __request } from '../core/request';
export class DefaultService {
    /**
     * Rebuild SQLite projection from markdown
     * @returns RebuildProjectionOutputBody OK
     * @throws ApiError
     */
    public static rebuildProjection(): CancelablePromise<RebuildProjectionOutputBody> {
        return __request(OpenAPI, {
            method: 'POST',
            url: '/admin/rebuild',
            errors: {
                500: `Internal Server Error`,
            },
        });
    }
    /**
     * Get client runtime config
     * @returns ClientConfigOutputBody OK
     * @returns ErrorModel Error
     * @throws ApiError
     */
    public static getClientConfig(): CancelablePromise<ClientConfigOutputBody | ErrorModel> {
        return __request(OpenAPI, {
            method: 'GET',
            url: '/client-config',
        });
    }
    /**
     * Get health
     * @returns HealthOutputBody OK
     * @returns ErrorModel Error
     * @throws ApiError
     */
    public static getHealth(): CancelablePromise<HealthOutputBody | ErrorModel> {
        return __request(OpenAPI, {
            method: 'GET',
            url: '/health',
        });
    }
    /**
     * List projects
     * @returns ListProjectsOutputBody OK
     * @throws ApiError
     */
    public static listProjects(): CancelablePromise<ListProjectsOutputBody> {
        return __request(OpenAPI, {
            method: 'GET',
            url: '/projects',
            errors: {
                500: `Internal Server Error`,
            },
        });
    }
    /**
     * Create project
     * @param requestBody
     * @returns Project Created
     * @throws ApiError
     */
    public static createProject(
        requestBody: CreateProjectRequest,
    ): CancelablePromise<Project> {
        return __request(OpenAPI, {
            method: 'POST',
            url: '/projects',
            body: requestBody,
            mediaType: 'application/json',
            errors: {
                400: `Bad Request`,
                409: `Conflict`,
                422: `Unprocessable Entity`,
                500: `Internal Server Error`,
            },
        });
    }
    /**
     * Delete project
     * @param project
     * @returns DeleteProjectOutputBody OK
     * @throws ApiError
     */
    public static deleteProject(
        project: string,
    ): CancelablePromise<DeleteProjectOutputBody> {
        return __request(OpenAPI, {
            method: 'DELETE',
            url: '/projects/{project}',
            path: {
                'project': project,
            },
            errors: {
                404: `Not Found`,
                422: `Unprocessable Entity`,
                500: `Internal Server Error`,
            },
        });
    }
    /**
     * List cards
     * @param project
     * @param includeDeleted
     * @returns ListCardsOutputBody OK
     * @throws ApiError
     */
    public static listCards(
        project: string,
        includeDeleted?: boolean,
    ): CancelablePromise<ListCardsOutputBody> {
        return __request(OpenAPI, {
            method: 'GET',
            url: '/projects/{project}/cards',
            path: {
                'project': project,
            },
            query: {
                'include_deleted': includeDeleted,
            },
            errors: {
                422: `Unprocessable Entity`,
                500: `Internal Server Error`,
            },
        });
    }
    /**
     * Create card
     * @param project
     * @param requestBody
     * @returns Card Created
     * @throws ApiError
     */
    public static createCard(
        project: string,
        requestBody: CreateCardRequest,
    ): CancelablePromise<Card> {
        return __request(OpenAPI, {
            method: 'POST',
            url: '/projects/{project}/cards',
            path: {
                'project': project,
            },
            body: requestBody,
            mediaType: 'application/json',
            errors: {
                400: `Bad Request`,
                422: `Unprocessable Entity`,
                500: `Internal Server Error`,
            },
        });
    }
    /**
     * Get card
     * @param project
     * @param number
     * @returns Card OK
     * @throws ApiError
     */
    public static getCard(
        project: string,
        number: number,
    ): CancelablePromise<Card> {
        return __request(OpenAPI, {
            method: 'GET',
            url: '/projects/{project}/cards/{number}',
            path: {
                'project': project,
                'number': number,
            },
            errors: {
                400: `Bad Request`,
                404: `Not Found`,
                422: `Unprocessable Entity`,
                500: `Internal Server Error`,
            },
        });
    }
    /**
     * Delete card
     * @param project
     * @param number
     * @param hard
     * @returns Card OK
     * @throws ApiError
     */
    public static deleteCard(
        project: string,
        number: number,
        hard?: boolean,
    ): CancelablePromise<Card> {
        return __request(OpenAPI, {
            method: 'DELETE',
            url: '/projects/{project}/cards/{number}',
            path: {
                'project': project,
                'number': number,
            },
            query: {
                'hard': hard,
            },
            errors: {
                400: `Bad Request`,
                404: `Not Found`,
                422: `Unprocessable Entity`,
                500: `Internal Server Error`,
            },
        });
    }
    /**
     * Set card branch metadata
     * @param project
     * @param number
     * @param requestBody
     * @returns Card OK
     * @throws ApiError
     */
    public static setCardBranch(
        project: string,
        number: number,
        requestBody: SetCardBranchRequest,
    ): CancelablePromise<Card> {
        return __request(OpenAPI, {
            method: 'PATCH',
            url: '/projects/{project}/cards/{number}/branch',
            path: {
                'project': project,
                'number': number,
            },
            body: requestBody,
            mediaType: 'application/json',
            errors: {
                400: `Bad Request`,
                404: `Not Found`,
                422: `Unprocessable Entity`,
                500: `Internal Server Error`,
            },
        });
    }
    /**
     * Append card comment
     * @param project
     * @param number
     * @param requestBody
     * @returns Card OK
     * @throws ApiError
     */
    public static commentCard(
        project: string,
        number: number,
        requestBody: TextBodyRequest,
    ): CancelablePromise<Card> {
        return __request(OpenAPI, {
            method: 'POST',
            url: '/projects/{project}/cards/{number}/comments',
            path: {
                'project': project,
                'number': number,
            },
            body: requestBody,
            mediaType: 'application/json',
            errors: {
                400: `Bad Request`,
                404: `Not Found`,
                422: `Unprocessable Entity`,
                500: `Internal Server Error`,
            },
        });
    }
    /**
     * Append card description entry
     * @param project
     * @param number
     * @param requestBody
     * @returns Card OK
     * @throws ApiError
     */
    public static appendDescription(
        project: string,
        number: number,
        requestBody: TextBodyRequest,
    ): CancelablePromise<Card> {
        return __request(OpenAPI, {
            method: 'PATCH',
            url: '/projects/{project}/cards/{number}/description',
            path: {
                'project': project,
                'number': number,
            },
            body: requestBody,
            mediaType: 'application/json',
            errors: {
                400: `Bad Request`,
                404: `Not Found`,
                422: `Unprocessable Entity`,
                500: `Internal Server Error`,
            },
        });
    }
    /**
     * Move card
     * @param project
     * @param number
     * @param requestBody
     * @returns Card OK
     * @throws ApiError
     */
    public static moveCard(
        project: string,
        number: number,
        requestBody: MoveCardRequest,
    ): CancelablePromise<Card> {
        return __request(OpenAPI, {
            method: 'PATCH',
            url: '/projects/{project}/cards/{number}/move',
            path: {
                'project': project,
                'number': number,
            },
            body: requestBody,
            mediaType: 'application/json',
            errors: {
                400: `Bad Request`,
                404: `Not Found`,
                422: `Unprocessable Entity`,
                500: `Internal Server Error`,
            },
        });
    }
    /**
     * Websocket event stream
     * Subscribe to project/card events. Optional project query param filters by project slug.
     * @returns void
     * @throws ApiError
     */
    public static websocketEvents(): CancelablePromise<void> {
        return __request(OpenAPI, {
            method: 'GET',
            url: '/ws',
        });
    }
}
