#ifdef _WIN32
#include <d3d11.h>
#include <dxgi1_2.h>
#include <windows.h>
#include <stdio.h>

extern "C" {

typedef struct {
    ID3D11Device* device;
    ID3D11DeviceContext* context;
    IDXGIOutputDuplication* duplication;
    ID3D11Texture2D* staging;
    int width;
    int height;
} DXGICapture;

DXGICapture* InitDXGI() {
    HRESULT hr;
    
    // Create D3D11 device
    ID3D11Device* device = nullptr;
    ID3D11DeviceContext* context = nullptr;
    D3D_FEATURE_LEVEL featureLevel;
    
    hr = D3D11CreateDevice(
        nullptr,
        D3D_DRIVER_TYPE_HARDWARE,
        nullptr,
        0,
        nullptr,
        0,
        D3D11_SDK_VERSION,
        &device,
        &featureLevel,
        &context
    );
    
    if (FAILED(hr)) {
        return nullptr;
    }
    
    // Get DXGI device
    IDXGIDevice* dxgiDevice = nullptr;
    hr = device->QueryInterface(__uuidof(IDXGIDevice), (void**)&dxgiDevice);
    if (FAILED(hr)) {
        device->Release();
        context->Release();
        return nullptr;
    }
    
    // Get DXGI adapter
    IDXGIAdapter* dxgiAdapter = nullptr;
    hr = dxgiDevice->GetAdapter(&dxgiAdapter);
    dxgiDevice->Release();
    if (FAILED(hr)) {
        device->Release();
        context->Release();
        return nullptr;
    }
    
    // Get primary output
    IDXGIOutput* dxgiOutput = nullptr;
    hr = dxgiAdapter->EnumOutputs(0, &dxgiOutput);
    dxgiAdapter->Release();
    if (FAILED(hr)) {
        device->Release();
        context->Release();
        return nullptr;
    }
    
    // Get output1 interface
    IDXGIOutput1* dxgiOutput1 = nullptr;
    hr = dxgiOutput->QueryInterface(__uuidof(IDXGIOutput1), (void**)&dxgiOutput1);
    dxgiOutput->Release();
    if (FAILED(hr)) {
        device->Release();
        context->Release();
        return nullptr;
    }
    
    // Create desktop duplication
    IDXGIOutputDuplication* duplication = nullptr;
    hr = dxgiOutput1->DuplicateOutput(device, &duplication);
    dxgiOutput1->Release();
    if (FAILED(hr)) {
        device->Release();
        context->Release();
        return nullptr;
    }
    
    // Get output description
    DXGI_OUTDUPL_DESC dupDesc;
    duplication->GetDesc(&dupDesc);
    
    int width = dupDesc.ModeDesc.Width;
    int height = dupDesc.ModeDesc.Height;
    
    // Create staging texture for CPU access
    D3D11_TEXTURE2D_DESC stagingDesc = {};
    stagingDesc.Width = width;
    stagingDesc.Height = height;
    stagingDesc.MipLevels = 1;
    stagingDesc.ArraySize = 1;
    stagingDesc.Format = DXGI_FORMAT_B8G8R8A8_UNORM;
    stagingDesc.SampleDesc.Count = 1;
    stagingDesc.Usage = D3D11_USAGE_STAGING;
    stagingDesc.CPUAccessFlags = D3D11_CPU_ACCESS_READ;
    
    ID3D11Texture2D* staging = nullptr;
    hr = device->CreateTexture2D(&stagingDesc, nullptr, &staging);
    if (FAILED(hr)) {
        duplication->Release();
        device->Release();
        context->Release();
        return nullptr;
    }
    
    // Allocate and return capture structure
    DXGICapture* cap = (DXGICapture*)malloc(sizeof(DXGICapture));
    cap->device = device;
    cap->context = context;
    cap->duplication = duplication;
    cap->staging = staging;
    cap->width = width;
    cap->height = height;
    
    return cap;
}

// Monitor info structure for enumeration
typedef struct {
    int index;
    int width;
    int height;
    int offsetX;
    int offsetY;
    int isPrimary;
    char name[64];
} MonitorInfoC;

// Enumerate all DXGI outputs (monitors)
int EnumDXGIOutputs(MonitorInfoC* infos, int maxCount) {
    HRESULT hr;

    // Create a temporary D3D11 device
    ID3D11Device* device = nullptr;
    ID3D11DeviceContext* context = nullptr;
    D3D_FEATURE_LEVEL featureLevel;

    hr = D3D11CreateDevice(
        nullptr, D3D_DRIVER_TYPE_HARDWARE, nullptr, 0,
        nullptr, 0, D3D11_SDK_VERSION,
        &device, &featureLevel, &context
    );
    if (FAILED(hr)) return 0;

    IDXGIDevice* dxgiDevice = nullptr;
    hr = device->QueryInterface(__uuidof(IDXGIDevice), (void**)&dxgiDevice);
    if (FAILED(hr)) { device->Release(); context->Release(); return 0; }

    IDXGIAdapter* adapter = nullptr;
    hr = dxgiDevice->GetAdapter(&adapter);
    dxgiDevice->Release();
    if (FAILED(hr)) { device->Release(); context->Release(); return 0; }

    int count = 0;
    IDXGIOutput* output = nullptr;
    for (int i = 0; adapter->EnumOutputs(i, &output) != DXGI_ERROR_NOT_FOUND && count < maxCount; i++) {
        DXGI_OUTPUT_DESC desc;
        output->GetDesc(&desc);

        infos[count].index = i;
        infos[count].width = desc.DesktopCoordinates.right - desc.DesktopCoordinates.left;
        infos[count].height = desc.DesktopCoordinates.bottom - desc.DesktopCoordinates.top;
        infos[count].offsetX = desc.DesktopCoordinates.left;
        infos[count].offsetY = desc.DesktopCoordinates.top;
        // Primary monitor is the one at 0,0
        infos[count].isPrimary = (desc.DesktopCoordinates.left == 0 && desc.DesktopCoordinates.top == 0) ? 1 : 0;
        // Convert wide name to ASCII
        for (int j = 0; j < 63 && desc.DeviceName[j]; j++) {
            infos[count].name[j] = (char)desc.DeviceName[j];
            infos[count].name[j+1] = 0;
        }

        count++;
        output->Release();
    }

    adapter->Release();
    device->Release();
    context->Release();
    return count;
}

// Initialize DXGI for a specific output index
DXGICapture* InitDXGIForOutput(int outputIndex) {
    HRESULT hr;

    ID3D11Device* device = nullptr;
    ID3D11DeviceContext* context = nullptr;
    D3D_FEATURE_LEVEL featureLevel;

    hr = D3D11CreateDevice(
        nullptr, D3D_DRIVER_TYPE_HARDWARE, nullptr, 0,
        nullptr, 0, D3D11_SDK_VERSION,
        &device, &featureLevel, &context
    );
    if (FAILED(hr)) return nullptr;

    IDXGIDevice* dxgiDevice = nullptr;
    hr = device->QueryInterface(__uuidof(IDXGIDevice), (void**)&dxgiDevice);
    if (FAILED(hr)) { device->Release(); context->Release(); return nullptr; }

    IDXGIAdapter* dxgiAdapter = nullptr;
    hr = dxgiDevice->GetAdapter(&dxgiAdapter);
    dxgiDevice->Release();
    if (FAILED(hr)) { device->Release(); context->Release(); return nullptr; }

    IDXGIOutput* dxgiOutput = nullptr;
    hr = dxgiAdapter->EnumOutputs(outputIndex, &dxgiOutput);
    dxgiAdapter->Release();
    if (FAILED(hr)) { device->Release(); context->Release(); return nullptr; }

    IDXGIOutput1* dxgiOutput1 = nullptr;
    hr = dxgiOutput->QueryInterface(__uuidof(IDXGIOutput1), (void**)&dxgiOutput1);
    dxgiOutput->Release();
    if (FAILED(hr)) { device->Release(); context->Release(); return nullptr; }

    IDXGIOutputDuplication* duplication = nullptr;
    hr = dxgiOutput1->DuplicateOutput(device, &duplication);
    dxgiOutput1->Release();
    if (FAILED(hr)) { device->Release(); context->Release(); return nullptr; }

    DXGI_OUTDUPL_DESC dupDesc;
    duplication->GetDesc(&dupDesc);

    int width = dupDesc.ModeDesc.Width;
    int height = dupDesc.ModeDesc.Height;

    D3D11_TEXTURE2D_DESC stagingDesc = {};
    stagingDesc.Width = width;
    stagingDesc.Height = height;
    stagingDesc.MipLevels = 1;
    stagingDesc.ArraySize = 1;
    stagingDesc.Format = DXGI_FORMAT_B8G8R8A8_UNORM;
    stagingDesc.SampleDesc.Count = 1;
    stagingDesc.Usage = D3D11_USAGE_STAGING;
    stagingDesc.CPUAccessFlags = D3D11_CPU_ACCESS_READ;

    ID3D11Texture2D* staging = nullptr;
    hr = device->CreateTexture2D(&stagingDesc, nullptr, &staging);
    if (FAILED(hr)) { duplication->Release(); device->Release(); context->Release(); return nullptr; }

    DXGICapture* cap = (DXGICapture*)malloc(sizeof(DXGICapture));
    cap->device = device;
    cap->context = context;
    cap->duplication = duplication;
    cap->staging = staging;
    cap->width = width;
    cap->height = height;

    return cap;
}

int CaptureDXGI(DXGICapture* cap, unsigned char* buffer, int bufferSize) {
    if (!cap || !buffer) {
        return -1;
    }
    
    HRESULT hr;
    IDXGIResource* desktopResource = nullptr;
    DXGI_OUTDUPL_FRAME_INFO frameInfo;
    
    // Acquire next frame
    hr = cap->duplication->AcquireNextFrame(100, &frameInfo, &desktopResource);
    if (FAILED(hr)) {
        if (hr == DXGI_ERROR_WAIT_TIMEOUT) {
            return 1; // No new frame (timeout)
        }
        return -2;
    }
    
    // Get texture from resource
    ID3D11Texture2D* acquiredTexture = nullptr;
    hr = desktopResource->QueryInterface(__uuidof(ID3D11Texture2D), (void**)&acquiredTexture);
    desktopResource->Release();
    if (FAILED(hr)) {
        cap->duplication->ReleaseFrame();
        return -3;
    }
    
    // Copy to staging texture
    cap->context->CopyResource(cap->staging, acquiredTexture);
    acquiredTexture->Release();
    
    // Map staging texture to CPU memory
    D3D11_MAPPED_SUBRESOURCE mappedResource;
    hr = cap->context->Map(cap->staging, 0, D3D11_MAP_READ, 0, &mappedResource);
    if (FAILED(hr)) {
        cap->duplication->ReleaseFrame();
        return -4;
    }
    
    // Copy pixel data
    int expectedSize = cap->width * cap->height * 4;
    if (bufferSize < expectedSize) {
        cap->context->Unmap(cap->staging, 0);
        cap->duplication->ReleaseFrame();
        return -5;
    }
    
    unsigned char* src = (unsigned char*)mappedResource.pData;
    int rowPitch = mappedResource.RowPitch;
    
    for (int y = 0; y < cap->height; y++) {
        memcpy(buffer + y * cap->width * 4, src + y * rowPitch, cap->width * 4);
    }
    
    // Unmap and release
    cap->context->Unmap(cap->staging, 0);
    cap->duplication->ReleaseFrame();
    
    return 0;
}

void CloseDXGI(DXGICapture* cap) {
    if (!cap) return;
    
    if (cap->staging) cap->staging->Release();
    if (cap->duplication) cap->duplication->Release();
    if (cap->context) cap->context->Release();
    if (cap->device) cap->device->Release();
    
    free(cap);
}

} // extern "C"
#endif // _WIN32
