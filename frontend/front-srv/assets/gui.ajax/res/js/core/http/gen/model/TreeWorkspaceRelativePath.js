/**
 * Pydio Cells Rest API
 * No description provided (generated by Swagger Codegen https://github.com/swagger-api/swagger-codegen)
 *
 * OpenAPI spec version: 1.0
 * 
 *
 * NOTE: This class is auto generated by the swagger code generator program.
 * https://github.com/swagger-api/swagger-codegen.git
 * Do not edit the class manually.
 *
 */


import ApiClient from '../ApiClient';





/**
* The TreeWorkspaceRelativePath model module.
* @module model/TreeWorkspaceRelativePath
* @version 1.0
*/
export default class TreeWorkspaceRelativePath {
    /**
    * Constructs a new <code>TreeWorkspaceRelativePath</code>.
    * @alias module:model/TreeWorkspaceRelativePath
    * @class
    */

    constructor() {
        

        
        

        

        
    }

    /**
    * Constructs a <code>TreeWorkspaceRelativePath</code> from a plain JavaScript object, optionally creating a new instance.
    * Copies all relevant properties from <code>data</code> to <code>obj</code> if supplied or a new instance if not.
    * @param {Object} data The plain JavaScript object bearing properties of interest.
    * @param {module:model/TreeWorkspaceRelativePath} obj Optional instance to populate.
    * @return {module:model/TreeWorkspaceRelativePath} The populated <code>TreeWorkspaceRelativePath</code> instance.
    */
    static constructFromObject(data, obj) {
        if (data) {
            obj = obj || new TreeWorkspaceRelativePath();

            
            
            

            if (data.hasOwnProperty('WsUuid')) {
                obj['WsUuid'] = ApiClient.convertToType(data['WsUuid'], 'String');
            }
            if (data.hasOwnProperty('WsLabel')) {
                obj['WsLabel'] = ApiClient.convertToType(data['WsLabel'], 'String');
            }
            if (data.hasOwnProperty('Path')) {
                obj['Path'] = ApiClient.convertToType(data['Path'], 'String');
            }
            if (data.hasOwnProperty('WsSlug')) {
                obj['WsSlug'] = ApiClient.convertToType(data['WsSlug'], 'String');
            }
            if (data.hasOwnProperty('WsScope')) {
                obj['WsScope'] = ApiClient.convertToType(data['WsScope'], 'String');
            }
        }
        return obj;
    }

    /**
    * @member {String} WsUuid
    */
    WsUuid = undefined;
    /**
    * @member {String} WsLabel
    */
    WsLabel = undefined;
    /**
    * @member {String} Path
    */
    Path = undefined;
    /**
    * @member {String} WsSlug
    */
    WsSlug = undefined;
    /**
    * @member {String} WsScope
    */
    WsScope = undefined;








}

