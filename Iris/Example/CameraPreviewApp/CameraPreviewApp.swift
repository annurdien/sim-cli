// CameraPreviewApp.swift
// Minimal SwiftUI example app demonstrating both Stage A (cooperative) and
// Stage B (transparent injection) modes of Iris.
//
// Stage A: Compile with IRIS_STAGE_A=1 — uses SharedMemoryCameraSource
// Stage B: Use normal AVFoundation; the injector dylib intercepts it.

import SwiftUI
import AVFoundation
import CoreMedia

@main
struct CameraPreviewApp: App {
    var body: some Scene {
        WindowGroup {
            ContentView()
        }
    }
}

// MARK: - Content View

struct ContentView: View {
    @StateObject private var camera = CameraController()

    var body: some View {
        ZStack {
            Color.black.ignoresSafeArea()

            CameraPreviewView(session: camera.session)
                .ignoresSafeArea()

            VStack {
                Spacer()
                HStack {
                    Image(systemName: "camera.fill")
                        .foregroundStyle(.white)
                    Text("Iris Preview")
                        .foregroundStyle(.white)
                        .font(.footnote.bold())
                    Spacer()
                    Text("Frame \(camera.frameCount)")
                        .foregroundStyle(.white.opacity(0.6))
                        .font(.footnote.monospaced())
                }
                .padding()
                .background(.ultraThinMaterial)
            }
        }
        .onAppear { camera.start() }
        .onDisappear { camera.stop() }
    }
}

// MARK: - Camera Preview Layer

class PreviewView: UIView {
    var videoPreviewLayer: AVCaptureVideoPreviewLayer {
        guard let layer = layer as? AVCaptureVideoPreviewLayer else {
            fatalError("Expected `AVCaptureVideoPreviewLayer` type for layer. Check PreviewView.layerClass implementation.")
        }
        return layer
    }
    
    var session: AVCaptureSession? {
        get { videoPreviewLayer.session }
        set { videoPreviewLayer.session = newValue }
    }
    
    override class var layerClass: AnyClass {
        return AVCaptureVideoPreviewLayer.self
    }
}

struct CameraPreviewView: UIViewRepresentable {
    let session: AVCaptureSession

    func makeUIView(context: Context) -> PreviewView {
        let view = PreviewView()
        view.backgroundColor = .clear
        view.videoPreviewLayer.videoGravity = .resizeAspectFill
        view.session = session
        return view
    }

    func updateUIView(_ uiView: PreviewView, context: Context) {
        uiView.session = session
    }
}

// MARK: - Camera Controller (Stage B: transparent AVFoundation)

@MainActor
final class CameraController: NSObject, ObservableObject {
    @Published var frameCount: Int = 0
    let session = AVCaptureSession()

    private let output  = AVCaptureVideoDataOutput()
    private let queue   = DispatchQueue(label: "com.example.camera.frames")

    func start() {
        session.sessionPreset = .hd1280x720

        if let device = AVCaptureDevice.default(for: .video),
           let input = try? AVCaptureDeviceInput(device: device),
           session.canAddInput(input) {
            session.addInput(input)
        }

        output.videoSettings = [
            kCVPixelBufferPixelFormatTypeKey as String: kCVPixelFormatType_32BGRA
        ]
        output.setSampleBufferDelegate(self, queue: queue)
        if session.canAddOutput(output) {
            session.addOutput(output)
        }

        session.startRunning()
    }

    func stop() {
        session.stopRunning()
    }
}

extension CameraController: AVCaptureVideoDataOutputSampleBufferDelegate {
    nonisolated func captureOutput(
        _ output: AVCaptureOutput,
        didOutput sampleBuffer: CMSampleBuffer,
        from connection: AVCaptureConnection
    ) {
        // We only use this delegate to increment the frame counter.
        // The actual rendering is handled by AVCaptureVideoPreviewLayer!
        Task { @MainActor in
            self.frameCount += 1
        }
    }

    nonisolated func captureOutput(
        _ output: AVCaptureOutput,
        didDrop sampleBuffer: CMSampleBuffer,
        from connection: AVCaptureConnection
    ) {
        // Dropped frames are acceptable in the MVP.
    }
}
