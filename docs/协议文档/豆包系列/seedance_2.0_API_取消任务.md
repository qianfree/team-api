`DELETE https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks/{id}`  [运行](https://api.volcengine.com/api-explorer/?action=DeleteContentsGenerationsTasks&data=%7B%22id%22%3A%22cgt-20250331175019-68d9t%22%7D&groupName=%E8%A7%86%E9%A2%91%E7%94%9F%E6%88%90API&query=%7B%7D&serviceCode=ark&version=2024-01-01)
取消排队中的视频生成任务，或者删除视频生成任务记录。

```mixin-react
return (<Tabs>
<Tabs.TabPane title="快速入口" key="vI631gwS"><RenderMd content={` [ ](#)[体验中心](https://console.volcengine.com/ark/region:ark+cn-beijing/experience/vision)       <span>![图片](https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_2abecd05ca2779567c6d32f0ddc7874d.png =20x) </span>[模型列表](https://www.volcengine.com/docs/82379/1330310)       <span>![图片](https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_a5fdd3028d35cc512a10bd71b982b6eb.png =20x) </span>[模型计费](https://www.volcengine.com/docs/82379/1099320#%E8%A7%86%E9%A2%91%E7%94%9F%E6%88%90%E6%A8%A1%E5%9E%8B)       <span>![图片](https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_afbcf38bdec05c05089d5de5c3fd8fc8.png =20x) </span>[API Key](https://console.volcengine.com/ark/region:ark+cn-beijing/apiKey?apikey=%7B%7D)
 <span>![图片](https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_57d0bca8e0d122ab1191b40101b5df75.png =20x) </span>[调用教程](https://www.volcengine.com/docs/82379/1366799)       <span>![图片](https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_f45b5cd5863d1eed3bc3c81b9af54407.png =20x) </span>[接口文档](https://www.volcengine.com/docs/82379/1521675)       <span>![图片](https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_1609c71a747f84df24be1e6421ce58f0.png =20x) </span>[常见问题](https://www.volcengine.com/docs/82379/1359411)       <span>![图片](https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_bef4bc3de3535ee19d0c5d6c37b0ffdd.png =20x) </span>[开通模型](https://console.volcengine.com/ark/region:ark+cn-beijing/openManagement?LLM=%7B%7D&OpenTokenDrawer=false)
`}></RenderMd></Tabs.TabPane>
<Tabs.TabPane title="鉴权说明" key="L8aMwmZD"><RenderMd content={`本接口支持 API Key 鉴权，详见[鉴权认证方式](https://www.volcengine.com/docs/82379/1298459)。
`}></RenderMd></Tabs.TabPane></Tabs>);
```


---


<span id="RxN8G2nH"></span>
## 请求参数
> 跳转 [响应参数](#7mi8G8RI)

:::tip
下面参数为Query String Parameters，在URL String中传入。

:::
---


**id** `string` %%require%%
需要取消或者删除的视频生成任务。
任务状态不同，调用`DELETE`接口，执行的操作有所不同，具体说明如下：

|当前任务状态  |是否支持DELETE操作 |操作含义  |DELETE操作后任务状态 |
|---|---|---|---|
|queued  |是  |任务取消排队，任务状态被变更为cancelled。  |cancelled  |
|running  |否 |\- |\- |
|succeeded  |是  |删除视频生成任务记录，后续将不支持查询。  |\- |
|failed  |是  |删除视频生成任务记录，后续将不支持查询。  |\- |
|cancelled  |否  |\- |\- |
|expired |是 |删除视频生成任务记录，后续将不支持查询。  |\- |


---


<span id="7mi8G8RI"></span>
## 响应参数
> 跳转 [请求参数](#RxN8G2nH)

本接口无返回参数。

