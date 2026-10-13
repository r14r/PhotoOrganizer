// Native USB transfer for photo-organizer, macOS 12+.
// Uses Apple's ImageCaptureCore rather than attempting to mount iOS photo storage.
#import <Foundation/Foundation.h>
#import <ImageCaptureCore/ImageCaptureCore.h>
#import <CommonCrypto/CommonDigest.h>
#include <stdio.h>

static NSString *HexSHA256(NSString *path) {
    NSInputStream *stream = [NSInputStream inputStreamWithFileAtPath:path];
    if (!stream) return nil;
    [stream open];
    CC_SHA256_CTX ctx; CC_SHA256_Init(&ctx);
    uint8_t buffer[65536]; NSInteger n;
    while ((n = [stream read:buffer maxLength:sizeof(buffer)]) > 0) CC_SHA256_Update(&ctx, buffer, (CC_LONG)n);
    [stream close];
    if (n < 0) return nil;
    unsigned char digest[CC_SHA256_DIGEST_LENGTH]; CC_SHA256_Final(digest, &ctx);
    NSMutableString *out = [NSMutableString string];
    for (int i = 0; i < CC_SHA256_DIGEST_LENGTH; i++) [out appendFormat:@"%02x", digest[i]];
    return out;
}

@interface PhoneImport : NSObject <ICDeviceBrowserDelegate, ICCameraDeviceDelegate, ICCameraDeviceDownloadDelegate>
@property(nonatomic, strong) ICDeviceBrowser *browser;
@property(nonatomic, strong) ICCameraDevice *camera;
@property(nonatomic, strong) NSArray<ICCameraFile *> *items;
@property(nonatomic, strong) NSMutableArray<ICCameraFile *> *verified;
@property(nonatomic, copy) NSString *destination;
@property(nonatomic, copy) NSString *deviceFilter;
@property(nonatomic, copy) NSString *staging;
@property(nonatomic, copy) NSString *temporaryName;
@property(nonatomic) NSUInteger nextIndex;
@property(nonatomic) BOOL listOnly;
@property(nonatomic) BOOL deleteAfterImport;
@property(nonatomic) BOOL didFail;
@property(nonatomic) BOOL done;
@property(nonatomic) BOOL started;
@property(nonatomic) BOOL deleteFailure;
@end

@implementation PhoneImport
- (void)fail:(NSString *)description {
    fprintf(stderr, "ERROR: %s\n", description.UTF8String);
    self.didFail = YES;
    self.done = YES;
}
- (void)deviceBrowser:(ICDeviceBrowser *)browser didAddDevice:(ICDevice *)device moreComing:(BOOL)moreComing { (void)browser; (void)device; (void)moreComing; }
- (void)deviceBrowser:(ICDeviceBrowser *)browser didRemoveDevice:(ICDevice *)device moreGoing:(BOOL)moreGoing {
    (void)browser; (void)moreGoing;
    if (device == self.camera && !self.done) [self fail:@"iPhone disconnected during import; nothing further will be deleted"];
}
- (void)deviceBrowserDidEnumerateLocalDevices:(ICDeviceBrowser *)browser {
    if (self.started) return;
    NSMutableArray<ICCameraDevice *> *candidates = [NSMutableArray array];
    for (ICDevice *device in browser.devices) {
        if (![device isKindOfClass:ICCameraDevice.class]) continue;
        if (self.deviceFilter.length > 0 && [device.name rangeOfString:self.deviceFilter options:NSCaseInsensitiveSearch].location == NSNotFound) continue;
        [candidates addObject:(ICCameraDevice *)device];
    }
    if (candidates.count == 0) { [self fail:@"No matching camera/iPhone found. Connect and unlock the phone, then trust the Mac."]; return; }
    if (candidates.count != 1) {
        fprintf(stderr, "Multiple devices found:\n");
        for (ICDevice *device in candidates) fprintf(stderr, "  %s\n", device.name.UTF8String);
        [self fail:@"Specify --device with part of the iPhone's name"];
        return;
    }
    self.camera = candidates.firstObject;
    self.started = YES;
    self.camera.delegate = self;
    printf("Device: %s\n", self.camera.name.UTF8String);
    [self.camera requestOpenSession];
}
- (void)device:(ICDevice *)device didOpenSessionWithError:(NSError *)error {
    (void)device;
    if (error) [self fail:[NSString stringWithFormat:@"Cannot open iPhone: %@", error.localizedDescription]];
}
- (void)device:(ICDevice *)device didCloseSessionWithError:(NSError *)error { (void)device; (void)error; }
- (void)deviceDidBecomeReadyWithCompleteContentCatalog:(ICCameraDevice *)camera {
    if (self.done) return;
    if (camera.accessRestrictedAppleDevice) { [self fail:@"Device is locked or not trusted"]; return; }
    if (self.deleteAfterImport && [camera.name rangeOfString:@"iPhone" options:NSCaseInsensitiveSearch].location == NSNotFound) {
        [self fail:@"Deletion allowed only for an identified iPhone, not a generic camera"];
        return;
    }
    if (self.deleteAfterImport && camera.iCloudPhotosEnabled) {
        [self fail:@"Deletion blocked: iCloud Photos is enabled on this iPhone. Import without --delete-after-import instead."];
        return;
    }
    NSMutableArray<ICCameraFile *> *files = [NSMutableArray array];
    for (ICCameraItem *item in camera.mediaFiles) {
        if ([item isKindOfClass:ICCameraFile.class]) [files addObject:(ICCameraFile *)item];
    }
    self.items = [files sortedArrayUsingComparator:^NSComparisonResult(ICCameraFile *a, ICCameraFile *b) {
        return [a.name localizedStandardCompare:b.name];
    }];
    self.verified = [NSMutableArray array];
    printf("Found %lu media files\n", (unsigned long)self.items.count);
    if (self.listOnly) {
        for (ICCameraFile *file in self.items) printf("%s (%llu bytes)\n", file.name.UTF8String, (unsigned long long)file.fileSize);
        self.done = YES;
        return;
    }
    if (self.items.count == 0) { self.done = YES; return; }
    self.staging = [self.destination stringByAppendingPathComponent:[NSString stringWithFormat:@".photo-organizer-import-%@", NSUUID.UUID.UUIDString]];
    NSError *error = nil;
    if (![NSFileManager.defaultManager createDirectoryAtPath:self.staging withIntermediateDirectories:YES attributes:nil error:&error]) {
        [self fail:[NSString stringWithFormat:@"Cannot create staging directory: %@", error.localizedDescription]];
        return;
    }
    [self downloadNext];
}
- (void)cameraDevice:(ICCameraDevice *)camera didAddItems:(NSArray<ICCameraItem *> *)items { (void)camera; (void)items; }
- (void)cameraDevice:(ICCameraDevice *)camera didRemoveItems:(NSArray<ICCameraItem *> *)items { (void)camera; (void)items; }
// Required ImageCaptureCore delegate methods (metadata/thumbnails are not requested).
- (void)didRemoveDevice:(ICDevice *)device {
    if (device == self.camera && !self.done) [self fail:@"iPhone disconnected during import"];
}
- (void)cameraDevice:(ICCameraDevice *)camera didRenameItems:(NSArray<ICCameraItem *> *)items { (void)camera; (void)items; }
- (void)cameraDevice:(ICCameraDevice *)camera didReceiveMetadata:(NSDictionary *)metadata forItem:(ICCameraItem *)item error:(NSError *)error {
    (void)camera; (void)metadata; (void)item; (void)error;
}
- (void)cameraDevice:(ICCameraDevice *)camera didReceiveThumbnail:(CGImageRef)thumbnail forItem:(ICCameraItem *)item error:(NSError *)error {
    (void)camera; (void)thumbnail; (void)item; (void)error;
}

- (NSString *)safeName:(NSString *)name {
    NSString *base = name.lastPathComponent;
    if (base.length == 0 || [base isEqualToString:@"."] || [base isEqualToString:@".."])
        return nil;
    return base;
}
- (NSString *)uniquePathForFile:(NSString *)name importedPath:(NSString *)importedPath {
    NSString *stem = [name stringByDeletingPathExtension];
    NSString *extension = [name pathExtension];
    NSFileManager *fs = NSFileManager.defaultManager;
    for (NSUInteger n = 0; n < 100000; n++) {
        NSString *candidate = n == 0 ? name : [NSString stringWithFormat:@"%@__%lu%@%@", stem, (unsigned long)n, extension.length ? @"." : @"", extension];
        NSString *path = [self.destination stringByAppendingPathComponent:candidate];
        if (![fs fileExistsAtPath:path]) return path;
        NSDictionary *existing = [fs attributesOfItemAtPath:path error:nil];
        NSDictionary *incoming = [fs attributesOfItemAtPath:importedPath error:nil];
        if ([existing[NSFileSize] isEqual:incoming[NSFileSize]]) {
            NSString *a = HexSHA256(path), *b = HexSHA256(importedPath);
            if (a && [a isEqualToString:b]) return path;
        }
    }
    return nil;
}
- (void)downloadNext {
    if (self.done) return;
    if (self.nextIndex >= self.items.count) {
        if (!self.deleteAfterImport) {
            printf("Import complete: %lu verified files; iPhone originals retained.\n", (unsigned long)self.verified.count);
            [NSFileManager.defaultManager removeItemAtPath:self.staging error:nil];
            self.done = YES;
            return;
        }
        // Only a fully verified batch can reach this point.
        if (self.verified.count != self.items.count || self.camera.iCloudPhotosEnabled || self.camera.locked) {
            [self fail:@"Deletion refused: incomplete transfer, iCloud Photos enabled, or device locked"];
            return;
        }
        printf("All %lu files verified locally. Requesting deletion on the iPhone...\n", (unsigned long)self.verified.count);
        [self.camera requestDeleteFiles:self.verified deleteFailed:^(NSDictionary *failures) {
            if (failures.count > 0) {
                self.deleteFailure = YES;
                fprintf(stderr, "ERROR: %lu device deletion failures\n", (unsigned long)failures.count);
            }
        } completion:^(NSDictionary *result, NSError *error) {
            (void)result;
            if (error || self.deleteFailure) {
                fprintf(stderr, "ERROR: iPhone deletion incomplete: %s\n", (error.localizedDescription ?: @"device rejected deletion").UTF8String);
                self.didFail = YES;
            } else {
                printf("iPhone deletion completed. Local copies remain intact.\n");
            }
            [NSFileManager.defaultManager removeItemAtPath:self.staging error:nil];
            self.done = YES;
        }];
        return;
    }
    ICCameraFile *file = self.items[self.nextIndex];
    NSString *name = [self safeName:file.name];
    if (!name) { [self fail:@"Unsafe filename from device"]; return; }
    self.temporaryName = [NSString stringWithFormat:@"%07lu-%@", (unsigned long)self.nextIndex, name];
    NSDictionary *options = @{
        ICDownloadsDirectoryURL: [NSURL fileURLWithPath:self.staging isDirectory:YES],
        ICSaveAsFilename: self.temporaryName,
        ICOverwrite: @NO,
        ICDownloadSidecarFiles: @NO
    };
    [self.camera requestDownloadFile:file options:options downloadDelegate:self didDownloadSelector:@selector(didDownloadFile:error:options:contextInfo:) contextInfo:NULL];
}
- (void)didDownloadFile:(ICCameraFile *)file error:(NSError *)error options:(NSDictionary *)options contextInfo:(void *)contextInfo {
    (void)contextInfo;
    if (self.done) return;
    if (error) { [self fail:[NSString stringWithFormat:@"Download failed (%@): %@", file.name, error.localizedDescription]]; return; }
    NSString *actualName = options[ICSavedFilename] ?: self.temporaryName;
    NSString *tempFile = [self.staging stringByAppendingPathComponent:actualName.lastPathComponent];
    NSDictionary *attributes = [NSFileManager.defaultManager attributesOfItemAtPath:tempFile error:&error];
    unsigned long long size = [attributes[NSFileSize] unsignedLongLongValue];
    if (error || !attributes || size == 0 || (file.fileSize > 0 && size != file.fileSize)) {
        [self fail:[NSString stringWithFormat:@"Invalid size for %@ (received %llu, expected %llu)", file.name, size, (unsigned long long)file.fileSize]];
        return;
    }
    NSString *name = [self safeName:file.name];
    NSString *dest = [self uniquePathForFile:name importedPath:tempFile];
    if (!dest) { [self fail:@"Cannot find free destination filename"]; return; }
    if ([NSFileManager.defaultManager fileExistsAtPath:dest]) {
        // A byte-identical local original already exists.
        [NSFileManager.defaultManager removeItemAtPath:tempFile error:nil];
        printf("VERIFIED EXISTING %s\n", dest.UTF8String);
    } else if (![NSFileManager.defaultManager moveItemAtPath:tempFile toPath:dest error:&error]) {
        [self fail:[NSString stringWithFormat:@"Cannot preserve local copy of %@: %@", name, error.localizedDescription]];
        return;
    } else {
        printf("IMPORTED %s\n", dest.UTF8String);
    }
    [self.verified addObject:file];
    self.nextIndex++;
    [self downloadNext];
}
- (void)run {
    self.browser = [ICDeviceBrowser new];
    self.browser.delegate = self;
    self.browser.browsedDeviceTypeMask = ICDeviceTypeMaskCamera;
    [self.browser start];
    NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:30];
    while (!self.done) {
        @autoreleasepool {
            [[NSRunLoop currentRunLoop] runMode:NSDefaultRunLoopMode beforeDate:[NSDate dateWithTimeIntervalSinceNow:0.2]];
            if (!self.started && [deadline timeIntervalSinceNow] < 0) [self fail:@"Timed out discovering iPhone (30 seconds)"];
        }
    }
    if (self.camera.hasOpenSession) [self.camera requestCloseSession];
    [self.browser stop];
}
@end

int main(int argc, const char **argv) {
    @autoreleasepool {
        PhoneImport *app = [PhoneImport new];
        for (int i = 1; i < argc; i++) {
            NSString *arg = [NSString stringWithUTF8String:argv[i]];
            if ([arg isEqualToString:@"--destination"] && i + 1 < argc) app.destination = [NSString stringWithUTF8String:argv[++i]];
            else if ([arg isEqualToString:@"--device"] && i + 1 < argc) app.deviceFilter = [NSString stringWithUTF8String:argv[++i]];
            else if ([arg isEqualToString:@"--list"]) app.listOnly = YES;
            else if ([arg isEqualToString:@"--delete-after-import"]) app.deleteAfterImport = YES;
            else if ([arg isEqualToString:@"--yes-delete"]) { /* explicitly validated with paired flag below */ }
            else { fprintf(stderr, "Unknown argument: %s\n", argv[i]); return 2; }
        }
        if (app.destination.length == 0) { fprintf(stderr, "--destination required\n"); return 2; }
        if (app.deleteAfterImport) {
            BOOL yes = NO;
            for (int i = 1; i < argc; i++) if (strcmp(argv[i], "--yes-delete") == 0) yes = YES;
            if (!yes || app.listOnly) { fprintf(stderr, "Deletion requires --delete-after-import and --yes-delete, without --list\n"); return 2; }
        }
        NSString *home = [app.destination stringByStandardizingPath];
        app.destination = home;
        if (!app.listOnly) {
            NSError *err = nil;
            if (![NSFileManager.defaultManager createDirectoryAtPath:app.destination withIntermediateDirectories:YES attributes:nil error:&err]) {
                fprintf(stderr, "Cannot create destination: %s\n", err.localizedDescription.UTF8String); return 1;
            }
        }
        [app run];
        return app.didFail ? 1 : 0;
    }
}
